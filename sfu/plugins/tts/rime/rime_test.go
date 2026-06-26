package rime

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/gokuljs/goSfu/plugins/tts"
	"github.com/joho/godotenv"
)

func TestMain(m *testing.M) {
	for _, p := range []string{"../../../.env", ".env"} {
		_ = godotenv.Load(p)
	}
	os.Exit(m.Run())
}

func TestProvider_NameAndSampleRate(t *testing.T) {
	p := New(Config{APIKey: "k", SampleRate: 22050})
	if p.Name() != "rime" {
		t.Fatalf("Name() = %q", p.Name())
	}
	if p.SampleRate() != 22050 {
		t.Fatalf("SampleRate() = %d, want 22050", p.SampleRate())
	}
}

// mockRimeServer speaks the ws3 protocol: it reads JSON messages and, on eos,
// replies with one base64 PCM chunk followed by a done event.
func mockRimeServer(t *testing.T, samples []int16) *httptest.Server {
	return mockRimeServerChunks(t, [][]byte{int16ToBytesLE(samples)})
}

func mockRimeServerChunks(t *testing.T, chunks [][]byte) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Errorf("Authorization = %q", got)
		}
		q := r.URL.Query()
		if q.Get("audioFormat") != "pcm" {
			t.Errorf("audioFormat = %q, want pcm", q.Get("audioFormat"))
		}
		if q.Get("segment") != "never" {
			t.Errorf("segment = %q, want never", q.Get("segment"))
		}

		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer c.Close(websocket.StatusNormalClosure, "")

		ctx := r.Context()
		for {
			_, data, err := c.Read(ctx)
			if err != nil {
				return
			}
			var msg map[string]any
			if json.Unmarshal(data, &msg) != nil {
				continue
			}
			if msg["operation"] == "eos" {
				for _, raw := range chunks {
					chunk, _ := json.Marshal(map[string]any{
						"type": "chunk",
						"data": base64.StdEncoding.EncodeToString(raw),
					})
					_ = c.Write(ctx, websocket.MessageText, chunk)
				}

				done, _ := json.Marshal(map[string]any{"type": "done"})
				_ = c.Write(ctx, websocket.MessageText, done)
				return
			}
		}
	}))
}

func TestSynthesize_streamsPCM(t *testing.T) {
	want := []int16{0, 100, -100, 32767, -32768, 42}
	srv := mockRimeServer(t, want)
	defer srv.Close()

	p := New(Config{
		APIKey:     "test-key",
		BaseURL:    "ws" + strings.TrimPrefix(srv.URL, "http"), // ws://127.0.0.1:port
		Model:      "mistv2",
		Speaker:    "cove",
		SampleRate: 24000,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ch, err := p.Synthesize(ctx, "hello world")
	if err != nil {
		t.Fatalf("Synthesize: %v", err)
	}

	var got []int16
	done := false
	for c := range ch {
		if c.Err != nil {
			t.Fatalf("chunk error: %v", c.Err)
		}
		if c.Done {
			done = true
			break
		}
		got = append(got, c.Samples...)
	}
	if !done {
		t.Fatal("expected Done chunk")
	}
	if len(got) != len(want) {
		t.Fatalf("got %d samples, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("sample[%d] = %d, want %d", i, got[i], want[i])
		}
	}
}

func TestSynthesize_preservesPCMAcrossOddByteChunks(t *testing.T) {
	want := []int16{0, 100, -100, 32767, -32768, 42, -900}
	raw := int16ToBytesLE(want)
	srv := mockRimeServerChunks(t, [][]byte{
		raw[:3],
		raw[3:8],
		raw[8:11],
		raw[11:],
	})
	defer srv.Close()

	p := New(Config{
		APIKey:     "test-key",
		BaseURL:    "ws" + strings.TrimPrefix(srv.URL, "http"),
		Model:      "mistv2",
		Speaker:    "cove",
		SampleRate: 24000,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ch, err := p.Synthesize(ctx, "hello world")
	if err != nil {
		t.Fatalf("Synthesize: %v", err)
	}

	var got []int16
	done := false
	for c := range ch {
		if c.Err != nil {
			t.Fatalf("chunk error: %v", c.Err)
		}
		if c.Done {
			done = true
			break
		}
		got = append(got, c.Samples...)
	}
	if !done {
		t.Fatal("expected Done chunk")
	}
	if len(got) != len(want) {
		t.Fatalf("got %d samples, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("sample[%d] = %d, want %d", i, got[i], want[i])
		}
	}
}

func TestBuild_requiresAPIKey(t *testing.T) {
	key := os.Getenv("RIME_API_KEY")
	os.Unsetenv("RIME_API_KEY")
	t.Cleanup(func() {
		if key != "" {
			os.Setenv("RIME_API_KEY", key)
		}
	})

	_, err := tts.Build("rime", tts.Options{})
	if err == nil || !strings.Contains(err.Error(), "API key") {
		t.Fatalf("expected API key error, got %v", err)
	}
}

// TestIntegration_Synthesize calls the real Rime API.
// Run: cd sfu && RIME_API_KEY=... go test -v ./plugins/tts/rime/ -run Integration
func TestIntegration_Synthesize(t *testing.T) {
	if os.Getenv("RIME_API_KEY") == "" {
		t.Skip("RIME_API_KEY not set; skipping integration test")
	}

	p, err := tts.Build("rime", tts.Options{})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	ch, err := p.Synthesize(ctx, "Hello from the voice agent.")
	if err != nil {
		t.Fatalf("Synthesize: %v", err)
	}

	var total int
	done := false
	for c := range ch {
		if c.Err != nil {
			t.Fatalf("chunk error: %v", c.Err)
		}
		if c.Done {
			done = true
			break
		}
		total += len(c.Samples)
	}
	if !done {
		t.Fatal("expected Done chunk")
	}
	if total == 0 {
		t.Fatal("expected non-empty PCM from Rime")
	}
	t.Logf("rime: received %d samples at %d Hz (%.2fs)", total, p.SampleRate(), float64(total)/float64(p.SampleRate()))
}

func int16ToBytesLE(s []int16) []byte {
	b := make([]byte, len(s)*2)
	for i, v := range s {
		binary.LittleEndian.PutUint16(b[2*i:], uint16(v))
	}
	return b
}
