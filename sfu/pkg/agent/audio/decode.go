package audio

import (
	"encoding/binary"
	"fmt"
	"io"
)

type Format int

const (
	FormatPCM Format = iota
	FormatWAV
)

// ReadPCMFrames reads s16le mono PCM and splits into 20ms frames.
func ReadPCMFrames(r io.Reader, sampleRate int) ([]Frame, error) {
	raw, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	if len(raw)%2 != 0 {
		return nil, fmt.Errorf("pcm: odd byte length")
	}

	samples := make([]int16, len(raw)/2)
	for i := range samples {
		samples[i] = int16(binary.LittleEndian.Uint16(raw[i*2:]))
	}

	frameSamples := sampleRate * 20 / 1000
	var frames []Frame
	for i := 0; i+frameSamples <= len(samples); i += frameSamples {
		f := Frame{
			Samples:    make([]int16, frameSamples),
			SampleRate: sampleRate,
		}
		copy(f.Samples, samples[i:i+frameSamples])
		frames = append(frames, f)
	}
	return frames, nil
}

// ReadWAVFrames reads a simple PCM WAV (s16le mono).
func ReadWAVFrames(r io.Reader) ([]Frame, error) {
	// Minimal WAV: parse fmt chunk, then data chunk.
	// For Phase 1, stub with header skip if you know format;
	// full parser can be added when TTS returns WAV.
	header := make([]byte, 44)
	if _, err := io.ReadFull(r, header); err != nil {
		return nil, err
	}
	sampleRate := int(binary.LittleEndian.Uint32(header[24:28]))
	return ReadPCMFrames(r, sampleRate)
}
