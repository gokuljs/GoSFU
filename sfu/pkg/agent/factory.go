package agent

import (
	"os"

	"github.com/gokuljs/goSfu/plugins/llm"
	"github.com/gokuljs/goSfu/plugins/stt"
	"github.com/gokuljs/goSfu/plugins/tts"
	"github.com/gokuljs/goSfu/plugins/vad"

	// Blank imports run each provider package's init(), which registers it by
	// name. Add real providers here as you build them, e.g.:
	//	_ "github.com/gokuljs/goSfu/plugins/stt/deepgram"
	//	_ "github.com/gokuljs/goSfu/plugins/llm/openai"
	//	_ "github.com/gokuljs/goSfu/plugins/tts/rime"
	//	_ "github.com/gokuljs/goSfu/plugins/vad/silero"
	_ "github.com/gokuljs/goSfu/plugins/llm/openai"
	_ "github.com/gokuljs/goSfu/plugins/llm/stub"
	_ "github.com/gokuljs/goSfu/plugins/stt/stub"
	_ "github.com/gokuljs/goSfu/plugins/tts/stub"
	_ "github.com/gokuljs/goSfu/plugins/vad/stub"
)

// DefaultAgentConfig resolves the plugin set from environment variables and
// returns a ready-to-use Config. The orchestrator never sees these env vars or
// any vendor detail — only the resulting interfaces.
//
//	STT_PROVIDER (default "stub")
//	LLM_PROVIDER (default "stub")
//	TTS_PROVIDER (default "stub")
//	VAD_PROVIDER (default "stub")
func DefaultAgentConfig() (Config, error) {
	sttP, err := stt.Build(envOr("STT_PROVIDER", "stub"))
	if err != nil {
		return Config{}, err
	}
	llmP, err := llm.Build(envOr("LLM_PROVIDER", "stub"))
	if err != nil {
		return Config{}, err
	}
	ttsP, err := tts.Build(envOr("TTS_PROVIDER", "stub"))
	if err != nil {
		return Config{}, err
	}
	vadP, err := vad.Build(envOr("VAD_PROVIDER", "stub"))
	if err != nil {
		return Config{}, err
	}

	return Config{
		Plugins: Plugins{
			STT: sttP,
			LLM: llmP,
			TTS: ttsP,
			VAD: vadP,
		},
		Settings: DefaultSettings(),
	}, nil
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
