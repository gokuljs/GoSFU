// Package rime implements tts.Provider using Rime's /ws3 JSON WebSocket API.
//
// Protocol (https://docs.rime.ai/docs/websockets):
//
//	connect: wss://users-ws.rime.ai/ws3?speaker=..&modelId=..&audioFormat=pcm&samplingRate=..&segment=never
//	header:  Authorization: Bearer <RIME_API_KEY>
//	send:    {"text": "<phrase>"}  then  {"operation": "eos"}
//	recv:    {"type":"chunk","data":"<base64 pcm>"} ... {"type":"done"}
//
// audioFormat=pcm yields raw mono signed 16-bit little-endian samples at the
// requested samplingRate. The orchestrator resamples that to 48kHz before
// sending to the transport, so SampleRate() must report the configured rate.
//
// One Synthesize call == one WebSocket connection == one phrase. The orchestrator
// already splits LLM output into sentences, so we use segment=never and emit a
// single eos to flush the whole phrase and close cleanly.
package rime

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"

	"github.com/coder/websocket"
	"github.com/gokuljs/goSfu/plugins/tts"
)

const (
	defaultBaseURL    = "wss://users-ws.rime.ai/ws3"
	defaultModel      = "mistv2"
	defaultSpeaker    = "cove"
	defaultSampleRate = 24000
	maxReadBytes      = 16 << 20 // base64 audio chunks can be large
)

func init() {
	tts.Register("rime", build)
}

func build(opts tts.Options) (tts.Provider, error) {
	key := opts.APIKey
	if key == "" {
		key = os.Getenv("RIME_API_KEY")
	}
	if key == "" {
		return nil, fmt.Errorf("rime: API key required (Options.APIKey or RIME_API_KEY)")
	}

	return New(Config{
		APIKey:     key,
		BaseURL:    envOr("RIME_WS_URL", defaultBaseURL),
		Model:      envOr("RIME_MODEL", defaultModel),
		Speaker:    firstNonEmpty(opts.Voice, os.Getenv("RIME_SPEAKER"), defaultSpeaker),
		SampleRate: envInt("RIME_SAMPLE_RATE", defaultSampleRate),
	}), nil
}

type Config struct {
	APIKey     string
	BaseURL    string // ws3 endpoint
	Model      string // modelId: coda | arcana | mistv1 | mistv2
	Speaker    string // voice; must be valid for the model
	SampleRate int    // 8000|16000|22050|24000|44100|48000|96000
}

type Provider struct {
	cfg Config
}

func New(cfg Config) *Provider {
	if cfg.BaseURL == "" {
		cfg.BaseURL = defaultBaseURL
	}
	if cfg.Model == "" {
		cfg.Model = defaultModel
	}
	if cfg.Speaker == "" {
		cfg.Speaker = defaultSpeaker
	}
	if cfg.SampleRate == 0 {
		cfg.SampleRate = defaultSampleRate
	}
	return &Provider{cfg: cfg}
}

func (p *Provider) Name() string    { return "rime" }
func (p *Provider) SampleRate() int { return p.cfg.SampleRate }

func (p *Provider) Synthesize(ctx context.Context, text string) (<-chan tts.Chunk, error) {
	hdr := http.Header{}
	hdr.Set("Authorization", "Bearer "+p.cfg.APIKey)

	conn, _, err := websocket.Dial(ctx, p.endpoint(), &websocket.DialOptions{HTTPHeader: hdr})
	if err != nil {
		return nil, fmt.Errorf("rime: dial: %w", err)
	}
	conn.SetReadLimit(maxReadBytes)

	// Send the phrase, then eos: synthesize the buffer, emit done, close.
	if err := writeJSON(ctx, conn, map[string]any{"text": text}); err != nil {
		conn.Close(websocket.StatusInternalError, "write text")
		return nil, fmt.Errorf("rime: send text: %w", err)
	}
	if err := writeJSON(ctx, conn, map[string]any{"operation": "eos"}); err != nil {
		conn.Close(websocket.StatusInternalError, "write eos")
		return nil, fmt.Errorf("rime: send eos: %w", err)
	}

	out := make(chan tts.Chunk)
	go func() {
		defer close(out)
		defer conn.Close(websocket.StatusNormalClosure, "")

		gotDone := false
		emit := func(c tts.Chunk) bool {
			select {
			case out <- c:
				return true
			case <-ctx.Done():
				return false
			}
		}

		for {
			_, data, err := conn.Read(ctx)
			if err != nil {
				if ctx.Err() != nil || gotDone {
					return
				}
				// Normal closure without an explicit done is still a clean end.
				if websocket.CloseStatus(err) == websocket.StatusNormalClosure {
					emit(tts.Chunk{Done: true})
					return
				}
				emit(tts.Chunk{Err: fmt.Errorf("rime: read: %w", err)})
				return
			}

			var msg wsMessage
			if json.Unmarshal(data, &msg) != nil {
				continue // ignore frames we don't understand
			}

			switch msg.Type {
			case "chunk":
				raw, derr := base64.StdEncoding.DecodeString(msg.Data)
				if derr != nil || len(raw) < 2 {
					continue
				}
				if !emit(tts.Chunk{Samples: bytesToInt16LE(raw)}) {
					return
				}
			case "error":
				emit(tts.Chunk{Err: fmt.Errorf("rime: %s", msg.Message)})
				return
			case "done":
				gotDone = true
				emit(tts.Chunk{Done: true})
				return
			default:
				// "timestamps" and any future event types are ignored.
			}
		}
	}()

	return out, nil
}

func (p *Provider) endpoint() string {
	q := url.Values{}
	q.Set("speaker", p.cfg.Speaker)
	q.Set("modelId", p.cfg.Model)
	q.Set("audioFormat", "pcm")
	q.Set("samplingRate", strconv.Itoa(p.cfg.SampleRate))
	q.Set("segment", "never")
	return p.cfg.BaseURL + "?" + q.Encode()
}

// wsMessage is the subset of the ws3 server event we consume.
type wsMessage struct {
	Type    string `json:"type"`
	Data    string `json:"data"`    // base64 PCM on "chunk"
	Message string `json:"message"` // on "error"
}

func writeJSON(ctx context.Context, conn *websocket.Conn, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return conn.Write(ctx, websocket.MessageText, b)
}

func bytesToInt16LE(b []byte) []int16 {
	n := len(b) / 2
	out := make([]int16, n)
	for i := 0; i < n; i++ {
		out[i] = int16(binary.LittleEndian.Uint16(b[2*i:]))
	}
	return out
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
