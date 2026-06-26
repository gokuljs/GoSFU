package audio

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/gokuljs/goSfu/pkg/logger"
)

// Pacer tuning. Mirrors LiveKit's AudioSource buffering strategy: a small jitter
// cushion absorbs slower-than-realtime / bursty TTS so the listener does not
// hear one-frame silence stutters, while a bounded buffer provides backpressure
// to the producer instead of dropping audio.
const (
	pacerPrebufferFrames   = 10 // 200ms cushion to build up before playout starts
	pacerStartTimeoutTicks = 8  // 160ms: start short utterances that never reach the cushion
	pacerMaxFrames         = 50 // 1s buffer ceiling -> upstream Send blocks past this
)

// FramePacer turns bursty PCM into a steady 20ms clock for WebRTC.
//
// Why a jitter buffer: TTS audio arrives in uneven bursts and can be slower than
// realtime. A naive "emit a frame if one is ready, else emit silence" pacer
// injects audible gaps on every underrun. Instead this pacer:
//
//   - Accumulates incoming samples in a bounded buffer (acc). The bound is the
//     backpressure mechanism: once full, top-up stops pulling, the inbound
//     channel fills, and the upstream Send blocks instead of dropping frames.
//   - Holds playout until the buffer has prebufferFrames worth of audio (a
//     cushion), or a short timeout fires for very short utterances.
//   - On underrun, re-enters buffering rather than emitting repeated single
//     silence frames.
//   - Exposes WaitForDrain so a caller can block until everything queued has
//     actually been emitted (clean half-duplex turn handoff), and Clear to drop
//     buffered audio for barge-in.
type FramePacer struct {
	in     <-chan Frame
	out    chan Frame
	drain  chan chan struct{}
	clear  chan struct{}
	roomID string

	onPlayoutStarted func(bufferedMs int)

	badRateOnce sync.Once
}

func NewFramePacer(in <-chan Frame, buf int, roomID string) *FramePacer {
	return &FramePacer{
		in:     in,
		out:    make(chan Frame, buf),
		drain:  make(chan chan struct{}),
		clear:  make(chan struct{}, 1),
		roomID: roomID,
	}
}

func (p *FramePacer) Out() <-chan Frame { return p.out }

func (p *FramePacer) SetOnPlayoutStarted(fn func(bufferedMs int)) {
	p.onPlayoutStarted = fn
}

// WaitForDrain blocks until all currently-buffered audio has been emitted to the
// output. Returns early if ctx is cancelled.
func (p *FramePacer) WaitForDrain(ctx context.Context) error {
	done := make(chan struct{})
	select {
	case p.drain <- done:
	case <-ctx.Done():
		return ctx.Err()
	}
	select {
	case <-done:
		logger.Pipeline(slog.LevelInfo, logger.EventPlayoutDrained,
			"Playout drained",
			"room", p.roomID,
		)
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Clear drops all buffered audio. Intended for barge-in / interruption so the
// agent stops playing a reply the user has talked over.
func (p *FramePacer) Clear() {
	select {
	case p.clear <- struct{}{}:
	default:
	}
}

func (p *FramePacer) Run(ctx context.Context) {
	ticker := time.NewTicker(FrameDuration)
	defer ticker.Stop()

	const (
		prebufferSamples = pacerPrebufferFrames * SamplesPerFrame48k
		maxSamples       = pacerMaxFrames * SamplesPerFrame48k
	)

	var (
		acc      []int16         // jitter buffer of pending PCM samples
		playing  bool            // false while building the cushion / idle
		wasPlay  bool            // track transitions for lifecycle logs
		bufTicks int             // ticks spent buffering with data (start timeout)
		waiters  []chan struct{} // pending WaitForDrain callers
	)

	closeWaiters := func() {
		for _, w := range waiters {
			close(w)
		}
		waiters = nil
	}

	for {
		select {
		case <-ctx.Done():
			closeWaiters()
			return

		case w := <-p.drain:
			// A drain request forces playout of whatever is buffered, ignoring
			// the prebuffer cushion, and releases the waiter once empty.
			waiters = append(waiters, w)

		case <-p.clear:
			bufferedMs := len(acc) * 1000 / WebrtcSampleRate
			drainedFrames := 0
			acc = acc[:0]
		drainIn:
			for {
				select {
				case <-p.in:
					drainedFrames++
				default:
					break drainIn
				}
			}
			playing = false
			wasPlay = false
			bufTicks = 0
			closeWaiters()
			logger.Pipeline(slog.LevelInfo, logger.EventPlayoutCleared,
				"Playout cleared",
				"room", p.roomID,
				"buffered_ms", bufferedMs,
				"drained_frames", drainedFrames,
			)

		case <-ticker.C:
			// Top up the jitter buffer from the inbound channel, bounded by
			// maxSamples so the producer blocks (backpressure) once we are full.
		topUp:
			for len(acc) < maxSamples {
				select {
				case f, ok := <-p.in:
					if !ok {
						closeWaiters()
						return
					}
					if f.SampleRate != WebrtcSampleRate && f.SampleRate != 0 {
						p.badRateOnce.Do(func() {
							slog.Warn("pacer got non-48k frame; upstream should have resampled",
								"room", p.roomID,
								"rate", f.SampleRate,
							)
						})
						continue
					}
					acc = append(acc, f.Samples...)
				default:
					break topUp
				}
			}

			draining := len(waiters) > 0

			// Decide whether to (re)start playout.
			if !playing {
				switch {
				case draining && len(acc) > 0:
					playing = true
					bufTicks = 0
				case len(acc) >= prebufferSamples:
					playing = true
					bufTicks = 0
				case len(acc) > 0:
					bufTicks++
					if bufTicks >= pacerStartTimeoutTicks {
						playing = true
						bufTicks = 0
					}
				default:
					bufTicks = 0
				}
			}

			if playing && !wasPlay {
				wasPlay = true
				bufferedMs := len(acc) * 1000 / WebrtcSampleRate
				logger.Pipeline(slog.LevelInfo, logger.EventPlayoutStarted,
					"Playout started",
					"room", p.roomID,
					"buffered_ms", bufferedMs,
				)
				if p.onPlayoutStarted != nil {
					p.onPlayoutStarted(bufferedMs)
				}
			}
			if !playing {
				wasPlay = false
			}

			switch {
			case playing && len(acc) >= SamplesPerFrame48k:
				f := Frame{Samples: make([]int16, SamplesPerFrame48k), SampleRate: WebrtcSampleRate}
				copy(f.Samples, acc[:SamplesPerFrame48k])
				acc = acc[SamplesPerFrame48k:]
				p.emit(ctx, f)

			case playing && len(acc) > 0:
				// Tail shorter than a full frame (end of an utterance): pad with
				// silence so we still flush it instead of holding it forever.
				f := NewSilentFrame48k()
				copy(f.Samples, acc)
				acc = acc[:0]
				p.emit(ctx, f)

			default:
				// Nothing ready: emit comfort silence and (re)enter buffering.
				if playing {
					logger.Pipeline(slog.LevelDebug, logger.EventPlayoutUnderrun,
						"Playout buffer underrun",
						"room", p.roomID,
					)
				}
				playing = false
				p.emit(ctx, NewSilentFrame48k())
			}

			if draining && len(acc) == 0 {
				closeWaiters()
			}
		}
	}
}

func (p *FramePacer) emit(ctx context.Context, f Frame) {
	select {
	case p.out <- f:
	case <-ctx.Done():
	}
}
