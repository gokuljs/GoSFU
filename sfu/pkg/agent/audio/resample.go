package audio

// Downsample48kTo16k: one 20ms frame at 48kHz → one 20ms frame at 16kHz.
// Phase 1: simple decimation (every 3rd sample). Upgrade later for quality.
func Downsample48kTo16k(in Frame) Frame {
	out := Frame{
		Samples:    make([]int16, SamplePerFrame16k),
		SampleRate: SttSampleRate,
	}
	for i := range out.Samples {
		out.Samples[i] = in.Samples[i*3]
	}
	return out
}

func Upsample16kTo48k(in Frame) Frame {
	out := Frame{
		Samples:    make([]int16, SamplesPerFrame48k),
		SampleRate: WebrtcSampleRate,
	}
	for i, s := range in.Samples {
		out.Samples[i*3] = s
		out.Samples[i*3+1] = s
		out.Samples[i*3+2] = s
	}
	return out
}

// Resample converts mono int16 PCM from one rate to another using linear
// interpolation. Good enough for voice; swap for a windowed-sinc later if
// you hear artifacts. Returns the original slice if rates already match.
func Resample(in []int16, fromRate, toRate int) []int16 {
	if fromRate == toRate || len(in) == 0 {
		return in
	}
	ratio := float64(fromRate) / float64(toRate)
	outLen := int(float64(len(in)) / ratio)
	out := make([]int16, outLen)
	for i := range out {
		src := float64(i) * ratio
		idx := int(src)
		frac := src - float64(idx)
		if idx+1 < len(in) {
			out[i] = int16(float64(in[idx])*(1-frac) + float64(in[idx+1])*frac)
		} else {
			out[i] = in[idx]
		}
	}
	return out
}
