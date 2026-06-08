// Inbound is the browser-mic entry point for the agent audio pipeline.
//
// Problem it solves: WebRTC delivers compressed Opus in variable-sized RTP
// packets, but downstream code (STT, resampling, level checks) wants raw PCM
// in fixed 20 ms chunks. Inbound sits between the WebRTC track and those
// consumers and performs that conversion on a dedicated goroutine.
package audio

import (
	"context"
	"log/slog"
	"sync/atomic"

	"github.com/gokuljs/goSfu/pkg/logger"
	"github.com/pion/webrtc/v4"
)

// defaultInboundQueue is the max number of frames that may wait for a consumer.
// Purpose: give STT (or similar) a short cushion (~500 ms) for jitter and brief
// pauses, without letting latency grow without bound if the consumer falls behind.
const defaultInboundQueue = 25

// Inbound owns the full path from remote track to PCM frames.
type Inbound struct {
	decoder *OpusDecoder  // fills the codec gap: Opus bytes → int16 PCM samples
	buffer  *SampleBuffer // fills the framing gap: variable PCM → 20 ms Frame values
	out     chan Frame    // fills the concurrency gap: Run produces, others consume
	roomID  string
}

// NewInbound wires decoder, reframer, and output queue for a standard browser
// mic track (48 kHz mono Opus).
func NewInbound(roomID string) (*Inbound, error) {
	dec, err := NewOpusDecoder(WebrtcSampleRate, ChannelsMono)
	if err != nil {
		return nil, err
	}
	return &Inbound{
		decoder: dec,
		buffer:  NewSampleBuffer(WebrtcSampleRate),
		out:     make(chan Frame, defaultInboundQueue),
		roomID:  roomID,
	}, nil
}

// Frames exposes the consumer side of the pipeline. Callers read from this
// channel instead of touching Run directly.
func (p *Inbound) Frames() <-chan Frame { return p.out }

// Run blocks on the WebRTC track, converts each RTP packet to PCM frames, and
// pushes them to Frames(). Start it in a goroutine; read from Frames() elsewhere.
func (p *Inbound) Run(ctx context.Context, track *webrtc.TrackRemote) {
	slog.Info("inbound audio started",
		"room", p.roomID,
		"codec", track.Codec().MimeType,
		"clockRate", track.Codec().ClockRate,
	)

	var (
		firstFrame   bool
		dropped      atomic.Uint64
		dropReported atomic.Bool
	)

	buf := make([]byte, 1500) // reuse buffer for each RTP read
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		n, _, err := track.Read(buf)
		if err != nil {
			slog.Info("inbound audio stopped", "room", p.roomID, "error", err)
			return
		}

		pcm, err := p.decoder.Decode(buf[:n])
		if err != nil {
			continue // corrupt packet — do not stall the track read loop
		}

		for _, frame := range p.buffer.Push(pcm) {
			if !firstFrame {
				firstFrame = true
				logger.Pipeline(slog.LevelInfo, logger.EventAudioFirstFrame,
					"Audio reached server — STT is processing your audio",
					"room", p.roomID,
					"codec", track.Codec().MimeType,
					"clock_rate", track.Codec().ClockRate,
				)
			}

			select {
			case p.out <- frame: // hand frame to consumer when queue has space
			case <-ctx.Done():
				return
			default: // consumer too slow — drop frame rather than block WebRTC reads
				dropped.Add(1)
				if !dropReported.Load() {
					dropReported.Store(true)
					logger.Pipeline(slog.LevelWarn, logger.EventAudioFrameDrop,
						"Inbound frame dropped — consumer too slow",
						"room", p.roomID,
						"dropped", dropped.Load(),
					)
				}
			}
		}
	}
}
