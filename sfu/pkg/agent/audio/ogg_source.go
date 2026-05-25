package agent

import (
	"context"
	"errors"
	"io"
	"os"
	"time"

	"github.com/pion/webrtc/v4"
	"github.com/pion/webrtc/v4/pkg/media"
	"github.com/pion/webrtc/v4/pkg/media/oggreader"
)

const oggPageDuration = 20 * time.Millisecond

func PlayOGG(ctx context.Context, path string, track *webrtc.TrackLocalStaticSample, stop <-chan struct{}) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	ogg, _, err := oggreader.NewWith(file)
	if err != nil {
		return err
	}

	var lastGranule uint64
	ticker := time.NewTicker(oggPageDuration)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-stop:
			return nil
		case <-ticker.C:
			pageData, pageHeader, err := ogg.ParseNextPage()
			if errors.Is(err, io.EOF) {
				if _, seekErr := file.Seek(0, io.SeekStart); seekErr != nil {
					return seekErr
				}
				ogg, _, err = oggreader.NewWith(file)
				if err != nil {
					return err
				}
				lastGranule = 0
				continue
			}
			if err != nil {
				return err
			}

			sampleCount := float64(pageHeader.GranulePosition - lastGranule)
			lastGranule = pageHeader.GranulePosition
			sampleDuration := time.Duration((sampleCount / 48000) * float64(time.Second))
			if sampleDuration <= 0 {
				sampleDuration = oggPageDuration
			}

			if err := track.WriteSample(media.Sample{
				Data:     pageData,
				Duration: sampleDuration,
			}); err != nil {
				return err
			}
		}
	}
}
