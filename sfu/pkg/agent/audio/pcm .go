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

// FlushPadded emits the final partial 20ms frame, padding the remainder with
// silence. This prevents callers from dropping the tail of a finite utterance.
func (b *SampleBuffer) FlushPadded() []Frame {
	if len(b.pending) == 0 {
		return nil
	}
	frameSamples := b.sampleRate * 20 / 1000
	f := Frame{
		Samples:    make([]int16, frameSamples),
		SampleRate: b.sampleRate,
	}
	copy(f.Samples, b.pending)
	b.pending = b.pending[:0]
	return []Frame{f}
}

// FadeInPlace applies a short linear ramp at the start of a PCM buffer. It is
// useful at synthetic utterance boundaries where the waveform may not begin
// close to zero.
func FadeInPlace(samples []int16, sampleRate, fadeMs int) {
	applyFade(samples, sampleRate, fadeMs, true)
}

// FadeOutPlace applies a short linear ramp at the end of a PCM buffer. It keeps
// a hard utterance boundary from ending on a non-zero waveform edge.
func FadeOutPlace(samples []int16, sampleRate, fadeMs int) {
	applyFade(samples, sampleRate, fadeMs, false)
}

func applyFade(samples []int16, sampleRate, fadeMs int, fadeIn bool) {
	if len(samples) == 0 || sampleRate <= 0 || fadeMs <= 0 {
		return
	}
	n := sampleRate * fadeMs / 1000
	if n <= 1 {
		return
	}
	if n > len(samples) {
		n = len(samples)
	}
	for i := 0; i < n; i++ {
		gain := float64(i) / float64(n-1)
		idx := i
		if !fadeIn {
			idx = len(samples) - 1 - i
		}
		samples[idx] = int16(math.Round(float64(samples[idx]) * gain))
	}
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
