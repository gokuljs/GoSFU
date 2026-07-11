package main

import (
	"encoding/binary"
	"fmt"
	"math"
	"os"

	"github.com/gokuljs/goSfu/pkg/agent/audio"
)

// audioSource produces 20 ms PCM frames at 48 kHz mono.
type audioSource interface {
	NextFrame() ([]int16, error)
}

type sineSource struct {
	freqHz     float64
	amplitude  float64
	sampleRate int
	position   int
}

func newSineSource(freqHz, amplitude float64) *sineSource {
	return &sineSource{
		freqHz:     freqHz,
		amplitude:  amplitude,
		sampleRate: audio.WebrtcSampleRate,
	}
}

func (s *sineSource) NextFrame() ([]int16, error) {
	frame := make([]int16, audio.SamplesPerFrame48k)
	for i := range frame {
		t := float64(s.position+i) / float64(s.sampleRate)
		v := s.amplitude * math.Sin(2*math.Pi*s.freqHz*t)
		frame[i] = int16(math.Round(v * 32767))
	}
	s.position += len(frame)
	return frame, nil
}

type wavSource struct {
	samples []int16
	cursor  int
}

func newWAVSource(path string) (*wavSource, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(data) < 44 {
		return nil, fmt.Errorf("wav %s: file too small", path)
	}
	if string(data[0:4]) != "RIFF" || string(data[8:12]) != "WAVE" {
		return nil, fmt.Errorf("wav %s: not a RIFF/WAVE file", path)
	}

	var (
		audioFormat   uint16
		channels      uint16
		sampleRate    uint32
		bitsPerSample uint16
		dataOffset    int
		dataSize      int
	)
	offset := 12
	for offset+8 <= len(data) {
		chunkID := string(data[offset : offset+4])
		chunkSize := int(binary.LittleEndian.Uint32(data[offset+4 : offset+8]))
		chunkStart := offset + 8
		chunkEnd := chunkStart + chunkSize
		if chunkEnd > len(data) {
			return nil, fmt.Errorf("wav %s: truncated chunk %s", path, chunkID)
		}

		switch chunkID {
		case "fmt ":
			if chunkSize < 16 {
				return nil, fmt.Errorf("wav %s: invalid fmt chunk", path)
			}
			audioFormat = binary.LittleEndian.Uint16(data[chunkStart : chunkStart+2])
			channels = binary.LittleEndian.Uint16(data[chunkStart+2 : chunkStart+4])
			sampleRate = binary.LittleEndian.Uint32(data[chunkStart+4 : chunkStart+8])
			bitsPerSample = binary.LittleEndian.Uint16(data[chunkStart+14 : chunkStart+16])
		case "data":
			dataOffset = chunkStart
			dataSize = chunkSize
		}
		offset = chunkEnd
		if offset%2 == 1 {
			offset++
		}
	}

	if audioFormat != 1 {
		return nil, fmt.Errorf("wav %s: only PCM is supported", path)
	}
	if channels != 1 {
		return nil, fmt.Errorf("wav %s: only mono is supported", path)
	}
	if bitsPerSample != 16 {
		return nil, fmt.Errorf("wav %s: only 16-bit PCM is supported", path)
	}
	if dataOffset == 0 || dataSize == 0 {
		return nil, fmt.Errorf("wav %s: missing data chunk", path)
	}

	raw := data[dataOffset : dataOffset+dataSize]
	samples := make([]int16, len(raw)/2)
	for i := range samples {
		samples[i] = int16(binary.LittleEndian.Uint16(raw[i*2 : i*2+2]))
	}

	if sampleRate != uint32(audio.WebrtcSampleRate) {
		samples = resampleLinear(samples, int(sampleRate), audio.WebrtcSampleRate)
	}

	return &wavSource{samples: samples}, nil
}

func (w *wavSource) NextFrame() ([]int16, error) {
	frame := make([]int16, audio.SamplesPerFrame48k)
	if len(w.samples) == 0 {
		return frame, nil
	}
	for i := range frame {
		frame[i] = w.samples[w.cursor%len(w.samples)]
		w.cursor++
	}
	return frame, nil
}

func resampleLinear(samples []int16, fromRate, toRate int) []int16 {
	if fromRate == toRate || len(samples) == 0 {
		return samples
	}
	outLen := int(float64(len(samples)) * float64(toRate) / float64(fromRate))
	if outLen == 0 {
		return nil
	}
	out := make([]int16, outLen)
	for i := range out {
		srcPos := float64(i) * float64(fromRate) / float64(toRate)
		idx := int(srcPos)
		frac := srcPos - float64(idx)
		a := float64(samples[idx%len(samples)])
		b := float64(samples[(idx+1)%len(samples)])
		out[i] = int16(math.Round(a + (b-a)*frac))
	}
	return out
}

func newAudioSource(wavPath string) (audioSource, error) {
	if wavPath != "" {
		return newWAVSource(wavPath)
	}
	return newSineSource(440, 0.35), nil
}
