package deepgram

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/gokuljs/goSfu/pkg/agent/audio"
	"github.com/gokuljs/goSfu/plugins/stt"
	"github.com/joho/godotenv"
)

func TestMain(m *testing.M) {
	for _, p := range []string{"../../../.env", ".env"} {
		_ = godotenv.Load(p)
	}
	os.Exit(m.Run())
}

func TestProvider_NameAndSampleRate(t *testing.T) {
	p := New(Config{APIKey: "k"})
	if p.Name() != "deepgram" {
		t.Fatalf("Name() = %q", p.Name())
	}
	if p.SampleRate() != audio.SttSampleRate {
		t.Fatalf("SampleRate() = %d, want %d", p.SampleRate(), audio.SttSampleRate)
	}
}

// mockDeepgram accepts the WS, waits for the first binary audio frame, then
// emits an interim and a final Results message.
func mockDeepgram(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Token test-key" {
			t.Errorf("Authorization = %q", got)
		}
		q := r.URL.Query()
		if q.Get("encoding") != "linear16" {
			t.Errorf("encoding = %q, want linear16", q.Get("encoding"))
		}
		if q.Get("sample_rate") != "16000" {
			t.Errorf("sample_rate = %q, want 16000", q.Get("sample_rate"))
		}

		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer c.Close(websocket.StatusNormalClosure, "")
		ctx := r.Context()

		// Wait for at least one audio frame from the client.
		if _, _, err := c.Read(ctx); err != nil {
			return
		}

		_ = c.Write(ctx, websocket.MessageText, results(false, "hello", 0.5))
		_ = c.Write(ctx, websocket.MessageText, results(true, "hello world", 0.98))
		// Give the client time to drain before the handler returns and closes.
		time.Sleep(100 * time.Millisecond)
	}))
}

func results(isFinal bool, transcript string, conf float64) []byte {
	b, _ := json.Marshal(map[string]any{
		"type":     "Results",
		"is_final": isFinal,
		"channel": map[string]any{
			"alternatives": []map[string]any{
				{"transcript": transcript, "confidence": conf},
			},
		},
	})
	return b
}

func TestSession_streamsResults(t *testing.T) {
	srv := mockDeepgram(t)
	defer srv.Close()

	p := New(Config{
		APIKey:     "test-key",
		BaseURL:    "ws" + strings.TrimPrefix(srv.URL, "http"),
		Model:      "nova-2",
		Language:   "en",
		SampleRate: audio.SttSampleRate,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sess, err := p.NewSession(ctx)
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	defer sess.Close()

	frame := audio.Frame{Samples: make([]int16, audio.SamplePerFrame16k), SampleRate: audio.SttSampleRate}
	if err := sess.SendFrame(ctx, frame); err != nil {
		t.Fatalf("SendFrame: %v", err)
	}

	var interim, final *stt.Result
	timeout := time.After(3 * time.Second)
	for final == nil {
		select {
		case r, ok := <-sess.Results():
			if !ok {
				t.Fatal("results channel closed before final")
			}
			rr := r
			if rr.IsFinal {
				final = &rr
			} else {
				interim = &rr
			}
		case <-timeout:
			t.Fatal("timed out waiting for final result")
		}
	}

	if interim == nil || interim.Text != "hello" {
		t.Fatalf("interim = %+v, want text 'hello'", interim)
	}
	if final.Text != "hello world" || final.Confidence != 0.98 {
		t.Fatalf("final = %+v, want 'hello world' conf 0.98", final)
	}
}

func TestBuild_requiresAPIKey(t *testing.T) {
	key := os.Getenv("DEEPGRAM_API_KEY")
	os.Unsetenv("DEEPGRAM_API_KEY")
	t.Cleanup(func() {
		if key != "" {
			os.Setenv("DEEPGRAM_API_KEY", key)
		}
	})

	_, err := stt.Build("deepgram", stt.Options{})
	if err == nil || !strings.Contains(err.Error(), "API key") {
		t.Fatalf("expected API key error, got %v", err)
	}
}

// TestIntegration_Session streams a short silence buffer to the real Deepgram
// API and verifies the stream opens and closes cleanly.
// Run: cd sfu && DEEPGRAM_API_KEY=... go test -v ./plugins/stt/deepgram/ -run Integration
func TestIntegration_Session(t *testing.T) {
	if os.Getenv("DEEPGRAM_API_KEY") == "" {
		t.Skip("DEEPGRAM_API_KEY not set; skipping integration test")
	}

	p, err := stt.Build("deepgram", stt.Options{})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	sess, err := p.NewSession(ctx)
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}

	// Send ~1s of silence; we only assert the stream stays healthy.
	frame := audio.Frame{Samples: make([]int16, audio.SamplePerFrame16k), SampleRate: audio.SttSampleRate}
	for i := 0; i < 50; i++ {
		if err := sess.SendFrame(ctx, frame); err != nil {
			t.Fatalf("SendFrame: %v", err)
		}
		time.Sleep(20 * time.Millisecond)
	}

	done := make(chan struct{})
	go func() {
		for r := range sess.Results() {
			t.Logf("deepgram result: final=%v text=%q", r.IsFinal, r.Text)
		}
		close(done)
	}()

	_ = sess.Close()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("results channel did not close after Close()")
	}
}
