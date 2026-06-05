// Package openai implements llm.Provider using the OpenAI Chat Completions API.
package openai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gokuljs/goSfu/plugins/llm"
)

func init() {
	llm.Register("openai", func() (llm.Provider, error) {
		key := os.Getenv("OPENAI_API_KEY")
		if key == "" {
			return nil, fmt.Errorf("openai: OPENAI_API_KEY is required")
		}
		return New(Config{
			APIKey:  key,
			Model:   envOr("OPENAI_MODEL", "gpt-4o-mini"),
			BaseURL: strings.TrimRight(envOr("OPENAI_BASE_URL", "https://api.openai.com/v1"), "/"),
		}), nil
	})
}

type Config struct {
	APIKey  string
	Model   string
	BaseURL string // for OpenAI-compatible proxies (optional)
}

type Provider struct {
	cfg Config
	cli *http.Client
}

func New(cfg Config) *Provider {
	return &Provider{
		cfg: cfg,
		cli: &http.Client{Timeout: 120 * time.Second},
	}
}

func (p *Provider) Name() string { return "openai" }

func (p *Provider) StreamCompletion(ctx context.Context, msgs []llm.Message) (<-chan llm.Chunk, error) {
	body, err := json.Marshal(chatRequest{
		Model:    p.cfg.Model,
		Messages: toOpenAIMessages(msgs),
		Stream:   true,
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.cfg.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+p.cfg.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.cli.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<10))
		resp.Body.Close()
		return nil, fmt.Errorf("openai: http %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}

	out := make(chan llm.Chunk)
	go func() {
		defer close(out)
		defer resp.Body.Close()

		sc := bufio.NewScanner(resp.Body)
		// SSE lines can be long
		sc.Buffer(make([]byte, 64*1024), 1024*1024)

		for sc.Scan() {
			line := strings.TrimSpace(sc.Text())
			if line == "" || !strings.HasPrefix(line, "data: ") {
				continue
			}
			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				select {
				case <-ctx.Done():
				case out <- llm.Chunk{Done: true}:
				}
				return
			}

			var chunk streamChunk
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				select {
				case <-ctx.Done():
				case out <- llm.Chunk{Err: fmt.Errorf("openai: parse chunk: %w", err)}:
				}
				return
			}
			if len(chunk.Choices) == 0 {
				continue
			}
			delta := chunk.Choices[0].Delta.Content
			if delta == "" {
				continue
			}
			select {
			case <-ctx.Done():
				return
			case out <- llm.Chunk{Delta: delta}:
			}
		}
		if err := sc.Err(); err != nil && ctx.Err() == nil {
			out <- llm.Chunk{Err: fmt.Errorf("openai: read stream: %w", err)}
			return
		}
		// Stream ended without [DONE]
		select {
		case <-ctx.Done():
		case out <- llm.Chunk{Done: true}:
		}
	}()

	return out, nil
}

// --- OpenAI API shapes (minimal) ---

type chatRequest struct {
	Model    string          `json:"model"`
	Messages []openAIMessage `json:"messages"`
	Stream   bool            `json:"stream"`
}

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type streamChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
	} `json:"choices"`
}

func toOpenAIMessages(msgs []llm.Message) []openAIMessage {
	out := make([]openAIMessage, len(msgs))
	for i, m := range msgs {
		out[i] = openAIMessage{Role: string(m.Role), Content: m.Content}
	}
	return out
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
