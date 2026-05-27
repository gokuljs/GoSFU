package audio

import "time"

const (
	WebrtcSampleRate = 48000 // because browser needs frequency in 48khz
	SttSampleRate    = 16000 // because stt needs frequency in 16khz
	ChannelsMono     = 1     // channels is mono

	FrameDuration      = 20 * time.Millisecond        // because webrtc needs frames in 20ms format
	SamplesPerFrame48k = WebrtcSampleRate * 20 / 1000 // here ware frame how many samples is required and doing that calculation here
	SamplePerFrame16k  = SttSampleRate * 20 / 1000
)

type Frame struct {
	Samples    []int16 // it stores pcm samples and they are in the format of int16
	SampleRate int     // tag for what the sameRate of the frame is
}

func NewSilentFrame48k() Frame {
	return Frame{
		Samples:    make([]int16, SamplesPerFrame48k),
		SampleRate: WebrtcSampleRate,
	}
}
