package vad

import (
	"context"

	"github.com/gokuljs/goSfu/pkg/agent/audio"
)

type EventType int

const (
	SpeechStart EventType = iota
	SpeechEnd
)

type Event struct {
	Type EventType
}

type Diagnostics struct {
	LastProbability  float32
	Speaking         bool
	SilentCount      int
	PendingSamples   int
	WindowSize       int
	SpeechThreshold  float32
	SilenceThreshold float32
}

type Provider interface {
	Name() string
	// SampleRate the model expects (Silero = 16000).
	SampleRate() int
	// Analyze feeds one frame and returns any state-change events (often empty).
	Analyze(ctx context.Context, frame audio.Frame) ([]Event, error)
	// Reset clears internal state between conversations.
	Reset()
	Close() error
}

type DiagnosticProvider interface {
	Diagnostics() Diagnostics
}
