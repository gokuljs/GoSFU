// This file contains helpers for working with raw PCM audio before it is sent
// through the agent pipeline. It keeps uneven audio chunks until they can be
// emitted as fixed 20ms frames and provides RMS for checking audio levels.
package audio

import "math"

// SampleBuffer exists to collect incoming PCM samples until there are enough
// samples to send as fixed 20ms audio frames.
type SampleBuffer struct {
	pending    []int16
	sampleRate int
}

// NewSampleBuffer prepares a buffer for audio at one sample rate so pushed
// samples can be split into correctly sized frames later.
func NewSampleBuffer(sampleRate int) *SampleBuffer {
	return &SampleBuffer{sampleRate: sampleRate}
}

func (b *SampleBuffer) SampleRate() int { return b.sampleRate }

// Push stores new PCM samples and returns every complete 20ms frame that can
// be built from the buffered audio.
func (b *SampleBuffer) Push(samples []int16) []Frame {
	b.pending = append(b.pending, samples...)
	frameSamples := b.sampleRate * 20 / 1000
	var out []Frame
	for len(b.pending) >= frameSamples {
		f := Frame{
			Samples:    make([]int16, frameSamples),
			SampleRate: b.sampleRate,
		}
		copy(f.Samples, b.pending[:frameSamples])
		out = append(out, f)
		b.pending = b.pending[frameSamples:]

	}
	return out
}

// RMS measures the average power of PCM samples, which is useful for checking
// audio volume levels or detecting silence.
func RMS(samples []int16) float64 {
	if len(samples) == 0 {
		return 0
	}
	var sum float64
	for _, s := range samples {
		v := float64(s)
		sum += v * v
	}
	return math.Sqrt(sum / float64(len(samples)))
}
