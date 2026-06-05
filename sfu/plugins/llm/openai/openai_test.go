package openai

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gokuljs/goSfu/plugins/llm"
	"github.com/joho/godotenv"
)

// TestMain loads sfu/.env before tests. go test runs with cwd = this package
// directory, so a plain godotenv.Load() would miss ../../../.env at module root.
func TestMain(m *testing.M) {
	for _, path := range []string{
		"../../../.env", // sfu/.env when cwd is plugins/llm/openai
		".env",            // sfu/.env when cwd is sfu/
	} {
		_ = godotenv.Load(path)
	}
	os.Exit(m.Run())
}

func TestProvider_Name(t *testing.T) {
	p := New(Config{APIKey: "k", Model: "gpt-4o-mini", BaseURL: "http://localhost"})
	if got := p.Name(); got != "openai" {
		t.Fatalf("Name() = %q, want openai", got)
	}
}

func TestStreamCompletion_streamsDeltas(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Errorf("path = %q, want /chat/completions", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Errorf("Authorization = %q", got)
		}

		var req chatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if !req.Stream {
			t.Fatal("expected stream=true")
		}
		if req.Model != "gpt-4o-mini" {
			t.Fatalf("model = %q", req.Model)
		}
		if len(req.Messages) != 1 || req.Messages[0].Content != "Hi" {
			t.Fatalf("messages = %+v", req.Messages)
		}

		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, ""+
			"data: {\"choices\":[{\"delta\":{\"content\":\"Hello\"}}]}\n\n"+
			"data: {\"choices\":[{\"delta\":{\"content\":\" world\"}}]}\n\n"+
			"data: [DONE]\n\n")
	}))
	defer srv.Close()

	p := New(Config{APIKey: "test-key", Model: "gpt-4o-mini", BaseURL: srv.URL})
	ch, err := p.StreamCompletion(context.Background(), []llm.Message{
		{Role: llm.RoleUser, Content: "Hi"},
	})
	if err != nil {
		t.Fatalf("StreamCompletion: %v", err)
	}

	got, done, err := collectChunks(ch)
	if err != nil {
		t.Fatalf("stream error: %v", err)
	}
	if !done {
		t.Fatal("expected Done chunk")
	}
	if got != "Hello world" {
		t.Fatalf("reply = %q, want %q", got, "Hello world")
	}
}

func TestStreamCompletion_httpError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"error":"invalid_api_key"}`, http.StatusUnauthorized)
	}))
	defer srv.Close()

	p := New(Config{APIKey: "bad", Model: "gpt-4o-mini", BaseURL: srv.URL})
	_, err := p.StreamCompletion(context.Background(), []llm.Message{
		{Role: llm.RoleUser, Content: "Hi"},
	})
	if err == nil {
		t.Fatal("expected error for 401")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Fatalf("error = %v, want http 401", err)
	}
}

func TestRegister_requiresAPIKey(t *testing.T) {
	key := os.Getenv("OPENAI_API_KEY")
	os.Unsetenv("OPENAI_API_KEY")
	t.Cleanup(func() {
		if key != "" {
			os.Setenv("OPENAI_API_KEY", key)
		}
	})

	// init() already registered "openai"; factory func reads env at call time.
	_, err := llm.Build("openai")
	if err == nil {
		t.Fatal("expected error when OPENAI_API_KEY is unset")
	}
	if !strings.Contains(err.Error(), "OPENAI_API_KEY") {
		t.Fatalf("error = %v", err)
	}
}

// TestIntegration_StreamCompletion calls the real OpenAI API.
// Skipped unless OPENAI_API_KEY is set in the environment.
//
// Run:
//
//	cd sfu && OPENAI_API_KEY=sk-... go test -v ./plugins/llm/openai/ -run Integration
func TestIntegration_StreamCompletion(t *testing.T) {
	key := os.Getenv("OPENAI_API_KEY")
	if key == "" {
		t.Skip("OPENAI_API_KEY not set; skipping integration test")
	}

	p, err := llm.Build("openai")
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	ch, err := p.StreamCompletion(context.Background(), []llm.Message{
		{Role: llm.RoleSystem, Content: "Reply in one short sentence."},
		{Role: llm.RoleUser, Content: "Say hello."},
	})
	if err != nil {
		t.Fatalf("StreamCompletion: %v", err)
	}

	reply, done, err := collectChunks(ch)
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	if !done {
		t.Fatal("expected Done chunk")
	}
	if strings.TrimSpace(reply) == "" {
		t.Fatal("expected non-empty reply from OpenAI")
	}
	t.Logf("openai reply: %s", reply)
}

func collectChunks(ch <-chan llm.Chunk) (text string, done bool, err error) {
	var b strings.Builder
	for c := range ch {
		if c.Err != nil {
			return b.String(), false, c.Err
		}
		if c.Done {
			done = true
			return b.String(), true, nil
		}
		b.WriteString(c.Delta)
	}
	return b.String(), done, nil
}
