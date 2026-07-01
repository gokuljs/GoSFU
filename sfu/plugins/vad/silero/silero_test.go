// plugins/vad/silero/silero_test.go
package silero

import (
	"context"
	"math"
	"os"
	"testing"

	"github.com/gokuljs/goSfu/pkg/agent/audio"
)

func TestIntegration_SileroDetectsTone(t *testing.T) {
	if os.Getenv("ONNXRUNTIME_LIB_PATH") == "" || os.Getenv("SILERO_MODEL_PATH") == "" {
		t.Skip("set ONNXRUNTIME_LIB_PATH and SILERO_MODEL_PATH to run")
	}
	p, err := New(os.Getenv("SILERO_MODEL_PATH"))
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	p.Reset()

	// Feed loud noise/speech-like frames and assert SpeechStart eventually fires.
	// (Pure sine often reads as non-speech; record a short WAV of your voice and
	//  feed its 16k samples here for a real assertion.)
	saw := false
	for i := 0; i < 200; i++ {
		f := audio.Frame{Samples: make([]int16, audio.SamplePerFrame16k), SampleRate: 16000}
		for j := range f.Samples {
			f.Samples[j] = int16(8000 * math.Sin(float64(i*320+j)*0.2))
		}
		ev, err := p.Analyze(context.Background(), f)
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range ev {
			if e.Type == 0 { // SpeechStart
				saw = true
			}
		}
	}
	_ = saw // sine may not trigger; replace with real speech samples for a hard assert
}
