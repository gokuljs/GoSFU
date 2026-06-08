// Package deepgram implements stt.Provider using Deepgram's streaming
// Listen WebSocket API (https://developers.deepgram.com/docs/streaming).
//
// Protocol:
//
//	connect: wss://api.deepgram.com/v1/listen?encoding=linear16&sample_rate=16000&channels=1&model=nova-2&interim_results=true
//	header:  Authorization: Token <DEEPGRAM_API_KEY>
//	send:    binary frames of raw linear16 (int16 LE) PCM
//	finish:  text {"type":"CloseStream"}
//	recv:    JSON {"type":"Results","is_final":bool,"channel":{"alternatives":[{"transcript","confidence"}]}}
//
// One conversation == one Session == one WebSocket. The orchestrator resamples
// mic audio to SampleRate() (16kHz) before calling SendFrame, so we forward the
// int16 samples as little-endian bytes with no further conversion.
package deepgram

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"sync"

	"github.com/coder/websocket"
	"github.com/gokuljs/goSfu/pkg/agent/audio"
	"github.com/gokuljs/goSfu/pkg/logger"
	"github.com/gokuljs/goSfu/plugins/stt"
)

const (
	defaultBaseURL  = "wss://api.deepgram.com/v1/listen"
	defaultModel    = "nova-2"
	defaultLanguage = "en"
	maxReadBytes    = 1 << 20
)

func init() {
	stt.Register("deepgram", build)
}

func build(opts stt.Options) (stt.Provider, error) {
	key := opts.APIKey
	if key == "" {
		key = os.Getenv("DEEPGRAM_API_KEY")
	}
	if key == "" {
		return nil, fmt.Errorf("deepgram: API key required (Options.APIKey or DEEPGRAM_API_KEY)")
	}

	model := opts.Model
	if model == "" {
		model = envOr("DEEPGRAM_MODEL", defaultModel)
	}

	return New(Config{
		APIKey:     key,
		BaseURL:    envOr("DEEPGRAM_WS_URL", defaultBaseURL),
		Model:      model,
		Language:   envOr("DEEPGRAM_LANGUAGE", defaultLanguage),
		SampleRate: audio.SttSampleRate, // 16000; orchestrator resamples to this
	}), nil
}

type Config struct {
	APIKey     string
	BaseURL    string
	Model      string
	Language   string
	SampleRate int
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
	if cfg.Language == "" {
		cfg.Language = defaultLanguage
	}
	if cfg.SampleRate == 0 {
		cfg.SampleRate = audio.SttSampleRate
	}
	return &Provider{cfg: cfg}
}

func (p *Provider) Name() string    { return "deepgram" }
func (p *Provider) SampleRate() int { return p.cfg.SampleRate }

func (p *Provider) NewSession(ctx context.Context) (stt.Session, error) {
	sctx, cancel := context.WithCancel(ctx)

	hdr := http.Header{}
	hdr.Set("Authorization", "Token "+p.cfg.APIKey)

	conn, _, err := websocket.Dial(sctx, p.endpoint(), &websocket.DialOptions{HTTPHeader: hdr})
	if err != nil {
		cancel()
		return nil, fmt.Errorf("deepgram: dial: %w", err)
	}
	conn.SetReadLimit(maxReadBytes)

	s := &session{
		conn:    conn,
		results: make(chan stt.Result, 16),
		cancel:  cancel,
	}
	go s.readLoop(sctx)
	return s, nil
}

func (p *Provider) endpoint() string {
	q := url.Values{}
	q.Set("encoding", "linear16")
	q.Set("sample_rate", strconv.Itoa(p.cfg.SampleRate))
	q.Set("channels", "1")
	q.Set("model", p.cfg.Model)
	q.Set("language", p.cfg.Language)
	q.Set("interim_results", "true")
	q.Set("punctuate", "true")
	q.Set("smart_format", "true")
	return p.cfg.BaseURL + "?" + q.Encode()
}

type session struct {
	conn      *websocket.Conn
	results   chan stt.Result
	cancel    context.CancelFunc
	closeOnce sync.Once
}

// SendFrame forwards one 16kHz PCM frame as little-endian linear16 bytes.
func (s *session) SendFrame(ctx context.Context, frame audio.Frame) error {
	return s.conn.Write(ctx, websocket.MessageBinary, int16ToBytesLE(frame.Samples))
}

func (s *session) Results() <-chan stt.Result { return s.results }

// Close tells Deepgram to finish, then cancels the read loop (which closes the
// connection and the results channel). Safe to call once.
func (s *session) Close() error {
	s.closeOnce.Do(func() {
		// Best-effort flush of any buffered audio before tearing down.
		_ = s.conn.Write(context.Background(), websocket.MessageText, []byte(`{"type":"CloseStream"}`))
		s.cancel()
	})
	return nil
}

func (s *session) readLoop(ctx context.Context) {
	defer close(s.results)
	defer s.conn.Close(websocket.StatusNormalClosure, "")

	for {
		_, data, err := s.conn.Read(ctx)
		if err != nil {
			return // ctx cancelled or connection closed
		}

		var msg dgMessage
		if json.Unmarshal(data, &msg) != nil {
			continue
		}
		// Only "Results" carry a channel; Metadata/UtteranceEnd/SpeechStarted don't.
		if msg.Channel == nil || len(msg.Channel.Alternatives) == 0 {
			continue
		}
		alt := msg.Channel.Alternatives[0]
		if alt.Transcript == "" {
			logger.Pipeline(slog.LevelDebug, logger.EventSTTEmptyResult,
				"Deepgram result with empty transcript",
				"is_final", msg.IsFinal,
				"speech_final", msg.SpeechFinal,
				"confidence", alt.Confidence,
			)
			continue
		}
		logger.Pipeline(slog.LevelDebug, logger.EventSTTResult,
			"Deepgram transcript result",
			"is_final", msg.IsFinal,
			"speech_final", msg.SpeechFinal,
			"confidence", alt.Confidence,
			"text_len", len(alt.Transcript),
			"text_preview", logger.Preview(alt.Transcript, 80),
		)

		select {
		case s.results <- stt.Result{
			Text:       alt.Transcript,
			IsFinal:    msg.IsFinal,
			Confidence: alt.Confidence,
		}:
		case <-ctx.Done():
			return
		}
	}
}

type dgMessage struct {
	Type        string `json:"type"`
	IsFinal     bool   `json:"is_final"`
	SpeechFinal bool   `json:"speech_final"`
	Channel     *struct {
		Alternatives []struct {
			Transcript string  `json:"transcript"`
			Confidence float64 `json:"confidence"`
		} `json:"alternatives"`
	} `json:"channel"`
}

func int16ToBytesLE(s []int16) []byte {
	b := make([]byte, len(s)*2)
	for i, v := range s {
		binary.LittleEndian.PutUint16(b[2*i:], uint16(v))
	}
	return b
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
