package agent

import (
	"log/slog"
	"os"
	"time"

	"github.com/gokuljs/goSfu/plugins/llm"
	"github.com/gokuljs/goSfu/plugins/stt"
	"github.com/gokuljs/goSfu/plugins/tts"
	"github.com/gokuljs/goSfu/plugins/vad"
)

// Options configures one agent instance. Pass explicit values for behavior and
// model settings; leave API keys empty to resolve them from the environment.
type Options struct {
	Settings Settings

	LLMProvider string
	LLM         llm.Options

	STTProvider string
	STT         stt.Options

	TTSProvider string
	TTS         tts.Options

	VADProvider string
	VAD         vad.Options
}

// NewConfig builds a ready-to-use Config from explicit options. The orchestrator
// only sees the resulting interfaces — never env vars or vendor details.
func NewConfig(opts Options) (Config, error) {
	configStart := time.Now()
	opts = opts.withDefaults()

	slog.Info("building STT provider", "provider", opts.STTProvider)
	t := time.Now()
	sttP, err := stt.Build(opts.STTProvider, opts.STT)
	if err != nil {
		return Config{}, err
	}
	slog.Info("STT provider ready", "provider", opts.STTProvider, "elapsed_ms", time.Since(t).Milliseconds())

	slog.Info("building LLM provider", "provider", opts.LLMProvider)
	t = time.Now()
	llmP, err := llm.Build(opts.LLMProvider, opts.LLM)
	if err != nil {
		return Config{}, err
	}
	slog.Info("LLM provider ready", "provider", opts.LLMProvider, "elapsed_ms", time.Since(t).Milliseconds())

	slog.Info("building TTS provider", "provider", opts.TTSProvider)
	t = time.Now()
	ttsP, err := tts.Build(opts.TTSProvider, opts.TTS)
	if err != nil {
		return Config{}, err
	}
	slog.Info("TTS provider ready", "provider", opts.TTSProvider, "elapsed_ms", time.Since(t).Milliseconds())

	slog.Info("building VAD provider", "provider", opts.VADProvider)
	t = time.Now()
	vadP, err := vad.Build(opts.VADProvider, opts.VAD)
	if err != nil {
		return Config{}, err
	}
	slog.Info("VAD provider ready", "provider", opts.VADProvider, "elapsed_ms", time.Since(t).Milliseconds())

	slog.Info("all providers ready", "total_ms", time.Since(configStart).Milliseconds())

	return Config{
		Plugins: Plugins{
			STT: sttP,
			LLM: llmP,
			TTS: ttsP,
			VAD: vadP,
		},
		Settings: opts.Settings,
	}, nil
}

// DefaultOptions returns options suitable for local development: provider names
// and API keys may be read from the environment; behavior defaults come from
// DefaultSettings() and each plugin's built-in defaults.
func DefaultOptions() Options {
	return Options{
		Settings:    DefaultSettings(),
		LLMProvider: envOr("LLM_PROVIDER", "stub"),
		STTProvider: envOr("STT_PROVIDER", "stub"),
		TTSProvider: envOr("TTS_PROVIDER", "stub"),
		VADProvider: envOr("VAD_PROVIDER", "stub"),
	}
}

// DefaultAgentConfig is a convenience wrapper around NewConfig(DefaultOptions()).
// Prefer NewConfig with explicit Options when creating per-room agents.
func DefaultAgentConfig() (Config, error) {
	return NewConfig(DefaultOptions())
}

func (o Options) withDefaults() Options {
	if o.LLMProvider == "" {
		o.LLMProvider = "stub"
	}
	if o.STTProvider == "" {
		o.STTProvider = "stub"
	}
	if o.TTSProvider == "" {
		o.TTSProvider = "stub"
	}
	if o.VADProvider == "" {
		o.VADProvider = "stub"
	}

	o.Settings = mergeSettings(o.Settings)
	return o
}

func mergeSettings(s Settings) Settings {
	d := DefaultSettings()
	if s.SystemPrompt != "" {
		d.SystemPrompt = s.SystemPrompt
	}
	if s.GreetingText != "" {
		d.GreetingText = s.GreetingText
	}
	if s.MaxChunkChars != 0 {
		d.MaxChunkChars = s.MaxChunkChars
	}
	if s.EndOfTurnSilence != 0 {
		d.EndOfTurnSilence = s.EndOfTurnSilence
	}
	return d
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
