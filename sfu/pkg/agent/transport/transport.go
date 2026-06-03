package transport

import (
	"context"

	"github.com/gokuljs/goSfu/pkg/agent/audio"
)

type Transport interface {
	// Inbound is decoded 20ms PCM frames from the remote peer (user).
	Inbound() <-chan audio.Frame
	// Send queues one 48kHz PCM frame to play to the remote peer (agent voice).
	// Non-blocking: drops if the playout buffer is full.
	send(frame audio.Frame) error
	// Start launches internal media goroutines.
	start(ctx context.Context) error
	close() error
}
