package audio

import "math"

// StreamResampler does continuous linear resampling across many chunks,
// carrying the fractional read position + leftover samples between calls so
// there is no per-chunk drift or boundary click. Any rate → any rate.
type StreamResampler struct {
	from, to int
	step     float64 // input samples advanced per output sample
	pos      float64 // fractional read cursor inside buf
	buf      []int16 // unconsumed input carried to next call
}

func NewStreamResampler(fromRate, toRate int) *StreamResampler {
	return &StreamResampler{from: fromRate, to: toRate, step: float64(fromRate) / float64(toRate)}
}

func (r *StreamResampler) Process(in []int16) []int16 {
	if r.from == r.to {
		return in
	}
	r.buf = append(r.buf, in...)
	var out []int16
	for {
		idx := int(r.pos)
		if idx+1 >= len(r.buf) {
			break
		}
		frac := r.pos - float64(idx)
		s := float64(r.buf[idx])*(1-frac) + float64(r.buf[idx+1])*frac
		out = append(out, int16(math.Round(s)))
		r.pos += r.step
	}
	if consumed := int(r.pos); consumed > 0 {
		r.buf = append(r.buf[:0], r.buf[consumed:]...)
		r.pos -= float64(consumed)
	}
	return out
}

func (r *StreamResampler) Flush() []int16 {
	if r.from == r.to || len(r.buf) == 0 {
		return nil
	}
	last := r.buf[len(r.buf)-1]
	r.buf = r.buf[:0]
	r.pos = 0
	return []int16{last}
}
