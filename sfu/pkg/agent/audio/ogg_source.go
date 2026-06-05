package audio

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"

	"github.com/pion/webrtc/v4/pkg/media/oggreader"
)

// OGGSource reads an Ogg/Opus file, decodes to PCM, pushes frames to out.
// This proves the outbound pipeline without TTS.
type OGGSource struct {
	path string
	dec  *OpusDecoder
}

func NewOGGSource(path string) (*OGGSource, error) {
	dec, err := NewOpusDecoder(WebrtcSampleRate, ChannelsMono)
	if err != nil {
		return nil, err
	}
	return &OGGSource{path: path, dec: dec}, nil
}

func (s *OGGSource) Run(ctx context.Context, out chan<- Frame) error {
	file, err := os.Open(s.path)
	if err != nil {
		return err
	}
	defer file.Close()

	ogg, _, err := oggreader.NewWith(file)
	if err != nil {
		return err
	}

	buffer := NewSampleBuffer(WebrtcSampleRate)
	slog.Info("ogg source started", "path", s.path)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		pageData, _, err := ogg.ParseNextPage()
		if errors.Is(err, io.EOF) {
			if _, seekErr := file.Seek(0, io.SeekStart); seekErr != nil {
				return seekErr
			}
			ogg, _, err = oggreader.NewWith(file)
			if err != nil {
				return err
			}
			continue
		}
		if err != nil {
			return err
		}

		pcm, err := s.dec.Decode(pageData)
		if err != nil {
			continue
		}

		for _, frame := range buffer.Push(pcm) {
			select {
			case out <- frame:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
}
