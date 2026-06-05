package agent

import (
	_ "github.com/gokuljs/goSfu/plugins/llm/openai"
	_ "github.com/gokuljs/goSfu/plugins/llm/stub"
	_ "github.com/gokuljs/goSfu/plugins/stt/deepgram"
	_ "github.com/gokuljs/goSfu/plugins/stt/stub"
	_ "github.com/gokuljs/goSfu/plugins/tts/rime"
	_ "github.com/gokuljs/goSfu/plugins/tts/stub"
	_ "github.com/gokuljs/goSfu/plugins/vad/stub"
)

// Blank imports run each provider package's init(), which registers it by name.
// Add real providers here as you build them, e.g.:
//
//	_ "github.com/gokuljs/goSfu/plugins/vad/silero"
