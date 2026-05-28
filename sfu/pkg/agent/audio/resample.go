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
