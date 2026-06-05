package stt

import (
	"context"

	"github.com/gokuljs/goSfu/pkg/agent/audio"
)

// Result is one transcription output. STT engines emit many interim
// (partial, unstable) results and occasional final (stable) ones.
type Result struct {
	Text       string
	IsFinal    bool
	Confidence float64
}

// Session is one live transcription stream. For Deepgram this wraps a
// single WebSocket. Lifecycle:
//	sess, _ := provider.NewSession(ctx)
//	go consume(sess.Results())
//	for frame := range mic { sess.SendFrame(ctx, frame) }
//	sess.Close()

type Session interface {
	// SendFrame pushes one PCM frame (at Provider.SampleRate()) to the engine.
	SendFrame(ctx context.Context, frame audio.Frame) error
	// Results streams transcripts until the session closes.
	Results() <-chan Result
	// Close ends the stream and releases the connection.
	Close() error
}

// Provider is the swappable engine. It is cheap and long-lived (holds API
// key + model). Sessions are created per conversation/turn.
type Provider interface {
	Name() string
	// SampleRate the engine expects (e.g. Deepgram = 16000).
	SampleRate() int
	NewSession(ctx context.Context) (Session, error)
}
