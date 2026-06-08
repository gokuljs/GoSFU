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

const defaultModel = "gpt-5.5"
const defaultBaseURL = "https://api.openai.com/v1"

func init() {
	llm.Register("openai", build)
}

func build(opts llm.Options) (llm.Provider, error) {
	key := opts.APIKey
	if key == "" {
		key = os.Getenv("OPENAI_API_KEY")
	}
	if key == "" {
		return nil, fmt.Errorf("openai: API key is required (pass Options.APIKey or set OPENAI_API_KEY)")
	}

	model := opts.Model
	if model == "" {
		model = defaultModel
	}

	baseURL := strings.TrimRight(opts.BaseURL, "/")
	if baseURL == "" {
		baseURL = defaultBaseURL
	}

	return New(Config{
		APIKey:      key,
		Model:       model,
		BaseURL:     baseURL,
		Temperature: opts.Temperature,
		Extra:       opts.Extra,
	}), nil
}

type Config struct {
	APIKey      string
	Model       string
	BaseURL     string // for OpenAI-compatible proxies (optional)
	Temperature *float64
	Extra       map[string]any // forwarded into the chat/completions JSON body
}

type Provider struct {
	cfg Config
	cli *http.Client
}

func New(cfg Config) *Provider {
	if cfg.Model == "" {
		cfg.Model = defaultModel
	}
	if strings.TrimRight(cfg.BaseURL, "/") == "" {
		cfg.BaseURL = defaultBaseURL
	} else {
		cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")
	}
	return &Provider{
		cfg: cfg,
		cli: &http.Client{Timeout: 120 * time.Second},
	}
}

func (p *Provider) Name() string { return "openai" }

func (p *Provider) StreamCompletion(ctx context.Context, msgs []llm.Message) (<-chan llm.Chunk, error) {
	body, err := marshalChatRequest(p.cfg, msgs)
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
			if chunk.Usage != nil {
				usage := &llm.Usage{
					PromptTokens:     chunk.Usage.PromptTokens,
					CompletionTokens: chunk.Usage.CompletionTokens,
					TotalTokens:      chunk.Usage.TotalTokens,
				}
				select {
				case <-ctx.Done():
					return
				case out <- llm.Chunk{Done: true, Usage: usage}:
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
	Model       string          `json:"model"`
	Messages    []openAIMessage `json:"messages"`
	Stream      bool            `json:"stream"`
	Temperature *float64        `json:"temperature,omitempty"`
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
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage,omitempty"`
}

func toOpenAIMessages(msgs []llm.Message) []openAIMessage {
	out := make([]openAIMessage, len(msgs))
	for i, m := range msgs {
		out[i] = openAIMessage{Role: string(m.Role), Content: m.Content}
	}
	return out
}

// marshalChatRequest builds the API body from typed fields, then merges Extra.
// Typed fields win over duplicate keys in Extra.
func marshalChatRequest(cfg Config, msgs []llm.Message) ([]byte, error) {
	payload := map[string]any{}
	for k, v := range cfg.Extra {
		payload[k] = v
	}
	payload["model"] = cfg.Model
	payload["messages"] = toOpenAIMessages(msgs)
	payload["stream"] = true
	payload["stream_options"] = map[string]any{"include_usage": true}
	if cfg.Temperature != nil {
		payload["temperature"] = *cfg.Temperature
	}
	return json.Marshal(payload)
}
