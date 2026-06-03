package agent

import (
	"time"

	vad "github.com/gokuljs/goSfu/plugins/Vad"
	"github.com/gokuljs/goSfu/plugins/llm"
	"github.com/gokuljs/goSfu/plugins/stt"
	"github.com/gokuljs/goSfu/plugins/tts"
)

// Plugins are the swappable capabilities, as interfaces. This is the struct
// your "agentOrchestrator = (stt=deepgram(...), llm=openai(...))" idea maps to.
type Plugins struct {
	STT stt.Provider
	LLM llm.Provider
	TTS tts.Provider
	VAD vad.Provider
}

// Settings are vendor-agnostic behavior knobs owned by the orchestrator.
type Settings struct {
	SystemPrompt     string
	GreetingText     string
	MaxChunkChars    int           // sentence-chunk fallback flush size
	EndOfTurnSilence time.Duration // informational; VAD owns the timer
	BargeIn          bool
}

type Config struct {
	Plugins  Plugins
	Settings Settings
}

func DefaultSettings() Settings {
	return Settings{
		SystemPrompt:     "You are a helpful, concise voice assistant.",
		GreetingText:     "Hi, how can I help you today?",
		MaxChunkChars:    160,
		EndOfTurnSilence: 800 * time.Millisecond,
		BargeIn:          true,
	}
}
