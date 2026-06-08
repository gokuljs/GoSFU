package transport

import (
	"context"

	"github.com/gokuljs/goSfu/pkg/agent/audio"
)

type Transport interface {
	// Inbound is decoded 20ms PCM frames from the remote peer (user).
	Inbound() <-chan audio.Frame
	// Send queues one 48kHz PCM frame to play to the remote peer (agent voice).
	// Backpressured: blocks while the playout buffer is full so audio is paced
	// to realtime instead of dropped. Returns ctx.Err() if ctx is cancelled.
	Send(ctx context.Context, frame audio.Frame) error
	// WaitForPlayout blocks until all queued audio has actually been played out,
	// so a half-duplex caller can hand the turn back only after the user has
	// heard the full reply.
	WaitForPlayout(ctx context.Context) error
	// ClearPlayout drops any buffered/queued audio (barge-in / interruption).
	ClearPlayout()
	// Start launches internal media goroutines.
	Start(ctx context.Context) error
	Close() error
}
