// This file bridges the agent's PCM audio pipeline and WebRTC's Opus codec.
// Browsers send and receive Opus over RTP, but internal processing (STT, level
// checks, resampling) needs raw PCM samples. These wrappers exist so the rest
// of the agent can work in PCM without caring about libopus details.
package audio

import "gopkg.in/hraban/opus.v2"

// OpusDecoder exists to turn incoming WebRTC/RTP Opus payloads into PCM that
// the agent can inspect, buffer, or forward to speech-to-text.
type OpusDecoder struct {
	dec *opus.Decoder
}

// NewOpusDecoder prepares a decoder for the sample rate and channel count used
// on the WebRTC track (typically 48 kHz mono).
func NewOpusDecoder(sampleRate, channels int) (*OpusDecoder, error) {
	dec, err := opus.NewDecoder(sampleRate, channels)
	if err != nil {
		return nil, err
	}
	return &OpusDecoder{dec: dec}, nil
}

// Decode expands one compressed Opus packet into raw int16 PCM samples.
// The output buffer is sized for up to 120 ms of audio because a single RTP
// packet can carry more than one 20 ms frame.
func (d *OpusDecoder) Decode(payload []byte) ([]int16, error) {
	pcm := make([]int16, SamplesPerFrame48k*6) // headroom for up to 120ms
	n, err := d.dec.Decode(payload, pcm)
	if err != nil {
		return nil, err
	}
	return pcm[:n], nil
}

// OpusEncoder exists to take PCM produced by the agent (e.g. TTS or processed
// audio) and compress it back into Opus for sending over WebRTC.
type OpusEncoder struct {
	enc *opus.Encoder
}

// NewOpusEncoder creates a speech-focused encoder tuned for real-time voice.
// AppVoIP tells libopus to optimize for conversational audio, and 32 kbps is a
// practical bitrate for clear speech without wasting bandwidth.
func NewOpusEncoder(sampleRate, channels int) (*OpusEncoder, error) {
	enc, err := opus.NewEncoder(sampleRate, channels, opus.AppVoIP)
	if err != nil {
		return nil, err
	}
	_ = enc.SetBitrate(32000)
	return &OpusEncoder{enc: enc}, nil
}

// Encode compresses one 20 ms PCM frame (960 samples at 48 kHz) into an Opus
// packet ready to be written into an RTP/WebRTC audio track.
func (e *OpusEncoder) Encode(pcm []int16) ([]byte, error) {
	out := make([]byte, 4000)
	n, err := e.enc.Encode(pcm, out)
	if err != nil {
		return nil, err
	}
	return out[:n], nil
}
