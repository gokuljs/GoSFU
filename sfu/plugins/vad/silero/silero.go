// Package silero implements vad.Provider using the Silero VAD v5 ONNX model
// via ONNX Runtime (CGO).
//
// The model needs:
//   - the ONNX Runtime shared library  (env ONNXRUNTIME_LIB_PATH)
//   - the silero_vad.onnx model file    (env SILERO_MODEL_PATH)
//
// It expects exactly 512 float32 samples per inference at 16kHz, and carries a
// recurrent state tensor (shape [2,1,128]) between calls. The orchestrator
// sends 20ms/320-sample frames, so we buffer and re-window to 512 internally.
package silero

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/gokuljs/goSfu/pkg/agent/audio"
	"github.com/gokuljs/goSfu/plugins/vad"
	ort "github.com/yalue/onnxruntime_go"
)

const (
	windowSize  = 512 // samples per inference @16k
	stateLen    = 2 * 1 * 128
	defaultRate = 16000

	speechThreshold  = 0.5  // prob >= this => speech
	silenceThreshold = 0.35 // prob < this  => silence (hysteresis)
	// 512 samples = 32ms per window. ~600ms silence => end of turn.
	minSilenceWindows = 19 // ceil(600 / 32)
)

// ortInit ensures the ONNX environment is initialized exactly once per process.
var ortInit sync.Once
var ortErr error

func initORT() error {
	ortInit.Do(func() {
		libPath := os.Getenv("ONNXRUNTIME_LIB_PATH")
		if libPath == "" {
			ortErr = fmt.Errorf("silero: ONNXRUNTIME_LIB_PATH not set")
			return
		}
		ort.SetSharedLibraryPath(libPath)
		ortErr = ort.InitializeEnvironment()
	})
	return ortErr
}

func init() {
	vad.Register("silero", func(_ vad.Options) (vad.Provider, error) {
		modelPath := os.Getenv("SILERO_MODEL_PATH")
		if modelPath == "" {
			return nil, fmt.Errorf("silero: SILERO_MODEL_PATH not set")
		}
		if err := initORT(); err != nil {
			return nil, err
		}
		return New(modelPath)
	})
}

// Provider is shared/long-lived; each NewSession-equivalent here is the Provider
// itself because the orchestrator keeps one VAD instance per call and calls
// Reset() at the start. State lives on the Provider.
type Provider struct {
	session *ort.DynamicAdvancedSession

	// reusable tensors (created once, data mutated per inference)
	inputT  *ort.Tensor[float32]
	stateT  *ort.Tensor[float32]
	srT     *ort.Tensor[int64]
	probT   *ort.Tensor[float32]
	nstateT *ort.Tensor[float32]

	pending []float32 // accumulates samples until >= windowSize

	speaking    bool
	silentCount int
	mu          sync.Mutex
}

func New(modelPath string) (*Provider, error) {
	// Create reusable tensors. Names below MUST match Step 2 output.
	inputT, err := ort.NewEmptyTensor[float32](ort.NewShape(1, windowSize))
	if err != nil {
		return nil, err
	}
	stateT, err := ort.NewEmptyTensor[float32](ort.NewShape(2, 1, 128))
	if err != nil {
		return nil, err
	}
	srT, err := ort.NewTensor(ort.NewShape(1), []int64{defaultRate})
	if err != nil {
		return nil, err
	}
	probT, err := ort.NewEmptyTensor[float32](ort.NewShape(1, 1))
	if err != nil {
		return nil, err
	}
	nstateT, err := ort.NewEmptyTensor[float32](ort.NewShape(2, 1, 128))
	if err != nil {
		return nil, err
	}

	session, err := ort.NewDynamicAdvancedSession(
		modelPath,
		[]string{"input", "state", "sr"}, // <-- from Step 2
		[]string{"output", "stateN"},     // <-- from Step 2
		nil,
	)
	if err != nil {
		return nil, err
	}

	return &Provider{
		session: session,
		inputT:  inputT,
		stateT:  stateT,
		srT:     srT,
		probT:   probT,
		nstateT: nstateT,
	}, nil
}

func (p *Provider) Name() string    { return "silero" }
func (p *Provider) SampleRate() int { return defaultRate }

func (p *Provider) Analyze(ctx context.Context, frame audio.Frame) ([]vad.Event, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// int16 -> normalized float32, append to pending.
	for _, s := range frame.Samples {
		p.pending = append(p.pending, float32(s)/32768.0)
	}

	var events []vad.Event
	for len(p.pending) >= windowSize {
		window := p.pending[:windowSize]
		p.pending = p.pending[windowSize:]

		prob, err := p.infer(window)
		if err != nil {
			return events, err
		}
		if ev, ok := p.step(prob); ok {
			events = append(events, ev)
		}
	}
	return events, nil
}

// infer runs one 512-sample window through the model and returns speech prob.
func (p *Provider) infer(window []float32) (float32, error) {
	copy(p.inputT.GetData(), window) // mutate reusable input tensor

	err := p.session.Run(
		[]ort.Value{p.inputT, p.stateT, p.srT},
		[]ort.Value{p.probT, p.nstateT},
	)
	if err != nil {
		return 0, fmt.Errorf("silero: run: %w", err)
	}

	// carry recurrent state forward
	copy(p.stateT.GetData(), p.nstateT.GetData())
	return p.probT.GetData()[0], nil
}

// step applies hysteresis + silence timer and emits transitions.
func (p *Provider) step(prob float32) (vad.Event, bool) {
	switch {
	case prob >= speechThreshold && !p.speaking:
		p.speaking = true
		p.silentCount = 0
		return vad.Event{Type: vad.SpeechStart}, true

	case prob >= silenceThreshold && p.speaking:
		// still speaking (or borderline) — reset silence timer
		p.silentCount = 0

	case prob < silenceThreshold && p.speaking:
		p.silentCount++
		if p.silentCount >= minSilenceWindows {
			p.speaking = false
			p.silentCount = 0
			return vad.Event{Type: vad.SpeechEnd}, true
		}
	}
	return vad.Event{}, false
}

func (p *Provider) Reset() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.speaking = false
	p.silentCount = 0
	p.pending = p.pending[:0]
	// zero the recurrent state
	for i := range p.stateT.GetData() {
		p.stateT.GetData()[i] = 0
	}
}

func (p *Provider) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.session != nil {
		p.session.Destroy()
		p.session = nil
	}
	p.inputT.Destroy()
	p.stateT.Destroy()
	p.srT.Destroy()
	p.probT.Destroy()
	p.nstateT.Destroy()
	return nil
}
