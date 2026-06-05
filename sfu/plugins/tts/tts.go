// Package tts is the Text-to-Speech plugin contract.
package tts

import "context"

// Chunk is a phase of synthesized audio. Engines emit several per sentence
// so playback can start before the whole sentence is rendered.
//
// Samples are mono int16 PCM AT THE PROVIDER'S NATIVE RATE (Provider.SampleRate()).
// The orchestrator resamples to 48kHz before sending to the transport.
type Chunk struct {
	Samples []int16
	Done    bool
	Err     error
}

type Provider interface {
	Name() string
	// SampleRate is the engine's native output rate (Rime ~22050, OpenAI 24000...).
	SampleRate() int
	// Synthesize streams audio for one text chunk (usually a sentence).
	Synthesize(ctx context.Context, text string) (<-chan Chunk, error)
}
