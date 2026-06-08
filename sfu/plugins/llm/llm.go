package llm

import "context"

type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

// Message is one turn of conversation. History is owned by the orchestrator,
// NOT by the plugin — the plugin is stateless per call.
type Message struct {
	Role    Role
	Content string
}

// Usage is token accounting from the provider (typically on the final chunk).
type Usage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

// Chunk is one streamed piece of the assistant's reply (token-ish).
type Chunk struct {
	Delta string // incremental text since the last chunk
	Done  bool   // true on the terminal chunk
	Err   error  // non-nil if the stream broke
	Usage *Usage // set on the terminal chunk when available
}

type Provider interface {
	Name() string
	// StreamCompletion streams the reply for the given message history.
	// The channel closes after a Chunk with Done=true (or Err set).
	StreamCompletion(ctx context.Context, msgs []Message) (<-chan Chunk, error)
}
