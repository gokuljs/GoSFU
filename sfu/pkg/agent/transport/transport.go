package transport

import (
	"context"
	"sync"
	"time"

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
	// SetPendingTTS records when a TTS utterance started for first-playable timing.
	SetPendingTTS(turn int, startedAt time.Time)
	// SetOnPlayoutStarted registers a callback fired when the pacer begins output.
	SetOnPlayoutStarted(fn func(turn int, bufferedMs int, elapsedMs int64))
	// Start launches internal media goroutines.
	Start(ctx context.Context) error
	Close() error
}

type pendingTTS struct {
	turn      int
	startedAt time.Time
}

type playoutBridge struct {
	mu       sync.Mutex
	pending  *pendingTTS
	callback func(turn int, bufferedMs int, elapsedMs int64)
}

func (b *playoutBridge) setPending(turn int, startedAt time.Time) {
	b.mu.Lock()
	b.pending = &pendingTTS{turn: turn, startedAt: startedAt}
	b.mu.Unlock()
}

func (b *playoutBridge) onPlayoutStarted(bufferedMs int) {
	b.mu.Lock()
	p := b.pending
	cb := b.callback
	if p != nil {
		b.pending = nil
	}
	b.mu.Unlock()
	if p == nil || cb == nil {
		return
	}
	cb(p.turn, bufferedMs, time.Since(p.startedAt).Milliseconds())
}

func (b *playoutBridge) setCallback(fn func(turn int, bufferedMs int, elapsedMs int64)) {
	b.mu.Lock()
	b.callback = fn
	b.mu.Unlock()
}
