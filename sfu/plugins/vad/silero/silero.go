// Package silero implements vad.Provider using the Silero VAD ONNX model
// via ONNX Runtime (direct CGO bindings).
//
// Reference: https://github.com/streamer45/silero-vad-go
//
// The model needs:
//   - ONNX Runtime library installed
//   - the silero_vad.onnx model file (env SILERO_MODEL_PATH)
//
// It expects 512 float32 samples per inference at 16kHz. After the first window,
// we prepend a 64-sample context from the previous window for continuity.
//
// For custom ONNX Runtime install paths, set CGO_CFLAGS and CGO_LDFLAGS:
//   CGO_CFLAGS="-I/path/to/include" CGO_LDFLAGS="-L/path/to/lib -lonnxruntime" go build ./...
package silero

// #cgo darwin,arm64 CFLAGS: -Wall -std=c99 -I/opt/homebrew/include
// #cgo darwin,arm64 LDFLAGS: -L/opt/homebrew/lib -lonnxruntime
// #cgo darwin,amd64 CFLAGS: -Wall -std=c99 -I/usr/local/include
// #cgo darwin,amd64 LDFLAGS: -L/usr/local/lib -lonnxruntime
// #cgo linux CFLAGS: -Wall -std=c99 -I/usr/local/include
// #cgo linux LDFLAGS: -L/usr/local/lib -lonnxruntime
// #include <stdlib.h>
// #include <string.h>
// #include <stdint.h>
// #include <onnxruntime/onnxruntime_c_api.h>
//
// static const OrtApi* gosfu_ort_api() {
//   return OrtGetApiBase()->GetApi(ORT_API_VERSION);
// }
//
// static void gosfu_release_status(const OrtApi* api, OrtStatus* status) {
//   if (status != NULL) api->ReleaseStatus(status);
// }
//
// static const char* gosfu_error_message(const OrtApi* api, OrtStatus* status) {
//   return api->GetErrorMessage(status);
// }
//
// static OrtStatus* gosfu_create_env(const OrtApi* api, OrtEnv** env) {
//   return api->CreateEnv(ORT_LOGGING_LEVEL_WARNING, "gosfu-vad", env);
// }
//
// static void gosfu_release_env(const OrtApi* api, OrtEnv* env) {
//   if (env != NULL) api->ReleaseEnv(env);
// }
//
// static OrtStatus* gosfu_create_session_options(const OrtApi* api, OrtSessionOptions** opts) {
//   return api->CreateSessionOptions(opts);
// }
//
// static void gosfu_release_session_options(const OrtApi* api, OrtSessionOptions* opts) {
//   if (opts != NULL) api->ReleaseSessionOptions(opts);
// }
//
// static OrtStatus* gosfu_set_threads(const OrtApi* api, OrtSessionOptions* opts) {
//   OrtStatus* status = api->SetIntraOpNumThreads(opts, 1);
//   if (status != NULL) return status;
//   status = api->SetInterOpNumThreads(opts, 1);
//   if (status != NULL) return status;
//   return api->SetSessionGraphOptimizationLevel(opts, ORT_ENABLE_ALL);
// }
//
// static OrtStatus* gosfu_create_session(const OrtApi* api, OrtEnv* env, const char* model_path, OrtSessionOptions* opts, OrtSession** session) {
//   return api->CreateSession(env, model_path, opts, session);
// }
//
// static void gosfu_release_session(const OrtApi* api, OrtSession* session) {
//   if (session != NULL) api->ReleaseSession(session);
// }
//
// static OrtStatus* gosfu_create_memory_info(const OrtApi* api, OrtMemoryInfo** memory_info) {
//   return api->CreateCpuMemoryInfo(OrtArenaAllocator, OrtMemTypeDefault, memory_info);
// }
//
// static void gosfu_release_memory_info(const OrtApi* api, OrtMemoryInfo* memory_info) {
//   if (memory_info != NULL) api->ReleaseMemoryInfo(memory_info);
// }
//
// static OrtStatus* gosfu_create_float_tensor(const OrtApi* api, const OrtMemoryInfo* memory_info, float* data, size_t len, int64_t* shape, size_t shape_len, OrtValue** value) {
//   return api->CreateTensorWithDataAsOrtValue(memory_info, data, len * sizeof(float), shape, shape_len, ONNX_TENSOR_ELEMENT_DATA_TYPE_FLOAT, value);
// }
//
// static OrtStatus* gosfu_create_int64_tensor(const OrtApi* api, const OrtMemoryInfo* memory_info, int64_t* data, size_t len, int64_t* shape, size_t shape_len, OrtValue** value) {
//   return api->CreateTensorWithDataAsOrtValue(memory_info, data, len * sizeof(int64_t), shape, shape_len, ONNX_TENSOR_ELEMENT_DATA_TYPE_INT64, value);
// }
//
// static OrtStatus* gosfu_run(const OrtApi* api, OrtSession* session, const char* const* input_names, const OrtValue* const* inputs, size_t input_count, const char* const* output_names, size_t output_count, OrtValue** outputs) {
//   return api->Run(session, NULL, input_names, inputs, input_count, output_names, output_count, outputs);
// }
//
// static OrtStatus* gosfu_tensor_data(const OrtApi* api, OrtValue* value, void** data) {
//   return api->GetTensorMutableData(value, data);
// }
//
// static void gosfu_release_value(const OrtApi* api, OrtValue* value) {
//   if (value != NULL) api->ReleaseValue(value);
// }
import "C"

import (
	"context"
	"fmt"
	"os"
	"sync"
	"unsafe"

	"github.com/gokuljs/goSfu/pkg/agent/audio"
	"github.com/gokuljs/goSfu/plugins/vad"
)

const (
	windowSize  = 512 // samples per inference @16k
	contextLen  = 64  // context samples prepended after first window
	stateLen    = 2 * 1 * 128
	defaultRate = 16000

	speechThreshold  = 0.5  // prob >= this => speech
	silenceThreshold = 0.35 // prob < this  => silence (hysteresis)
	minSilenceMs     = 100  // ms of silence before ending speech
	speechPadMs      = 30   // padding around speech segments
)

func init() {
	vad.Register("silero", func(_ vad.Options) (vad.Provider, error) {
		modelPath := os.Getenv("SILERO_MODEL_PATH")
		if modelPath == "" {
			return nil, fmt.Errorf("silero: SILERO_MODEL_PATH not set")
		}
		return New(modelPath)
	})
}

type Provider struct {
	api         *C.OrtApi
	env         *C.OrtEnv
	sessionOpts *C.OrtSessionOptions
	session     *C.OrtSession
	memoryInfo  *C.OrtMemoryInfo
	cStrings    map[string]*C.char

	state [stateLen]float32
	ctx   [contextLen]float32

	pending []float32

	currSample int
	triggered  bool
	tempEnd    int
	lastProb   float32
	mu         sync.Mutex
}

func New(modelPath string) (*Provider, error) {
	if _, err := os.Stat(modelPath); err != nil {
		return nil, fmt.Errorf("silero: model file not found at %q (run scripts/download-model.sh): %w", modelPath, err)
	}

	p := &Provider{
		api:      C.gosfu_ort_api(),
		cStrings: make(map[string]*C.char),
	}
	if p.api == nil {
		return nil, fmt.Errorf("silero: failed to get ONNX Runtime API")
	}

	if status := C.gosfu_create_env(p.api, &p.env); status != nil {
		defer C.gosfu_release_status(p.api, status)
		return nil, fmt.Errorf("silero: create env: %s", C.GoString(C.gosfu_error_message(p.api, status)))
	}

	if status := C.gosfu_create_session_options(p.api, &p.sessionOpts); status != nil {
		defer C.gosfu_release_status(p.api, status)
		p.Close()
		return nil, fmt.Errorf("silero: create session options: %s", C.GoString(C.gosfu_error_message(p.api, status)))
	}

	if status := C.gosfu_set_threads(p.api, p.sessionOpts); status != nil {
		defer C.gosfu_release_status(p.api, status)
		p.Close()
		return nil, fmt.Errorf("silero: configure session: %s", C.GoString(C.gosfu_error_message(p.api, status)))
	}

	p.cStrings["modelPath"] = C.CString(modelPath)
	if status := C.gosfu_create_session(p.api, p.env, p.cStrings["modelPath"], p.sessionOpts, &p.session); status != nil {
		defer C.gosfu_release_status(p.api, status)
		p.Close()
		return nil, fmt.Errorf("silero: create session: %s", C.GoString(C.gosfu_error_message(p.api, status)))
	}

	if status := C.gosfu_create_memory_info(p.api, &p.memoryInfo); status != nil {
		defer C.gosfu_release_status(p.api, status)
		p.Close()
		return nil, fmt.Errorf("silero: create memory info: %s", C.GoString(C.gosfu_error_message(p.api, status)))
	}

	for _, name := range []string{"input", "state", "sr", "output", "stateN"} {
		p.cStrings[name] = C.CString(name)
	}

	return p, nil
}

func (p *Provider) Name() string    { return "silero" }
func (p *Provider) SampleRate() int { return defaultRate }

func (p *Provider) Analyze(ctx context.Context, frame audio.Frame) ([]vad.Event, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, s := range frame.Samples {
		p.pending = append(p.pending, float32(s)/32768.0)
	}

	var events []vad.Event
	for len(p.pending) >= windowSize {
		window := append([]float32(nil), p.pending[:windowSize]...)
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

func (p *Provider) infer(window []float32) (float32, error) {
	pcm := window
	if p.currSample > 0 {
		pcm = append(p.ctx[:], window...)
	}
	copy(p.ctx[:], window[len(window)-contextLen:])

	inputShape := []C.int64_t{1, C.int64_t(len(pcm))}
	var inputValue *C.OrtValue
	if status := C.gosfu_create_float_tensor(
		p.api,
		p.memoryInfo,
		(*C.float)(unsafe.Pointer(&pcm[0])),
		C.size_t(len(pcm)),
		(*C.int64_t)(unsafe.Pointer(&inputShape[0])),
		C.size_t(len(inputShape)),
		&inputValue,
	); status != nil {
		defer C.gosfu_release_status(p.api, status)
		return 0, fmt.Errorf("silero: create input tensor: %s", C.GoString(C.gosfu_error_message(p.api, status)))
	}
	defer C.gosfu_release_value(p.api, inputValue)

	stateShape := []C.int64_t{2, 1, 128}
	var stateValue *C.OrtValue
	if status := C.gosfu_create_float_tensor(
		p.api,
		p.memoryInfo,
		(*C.float)(unsafe.Pointer(&p.state[0])),
		C.size_t(len(p.state)),
		(*C.int64_t)(unsafe.Pointer(&stateShape[0])),
		C.size_t(len(stateShape)),
		&stateValue,
	); status != nil {
		defer C.gosfu_release_status(p.api, status)
		return 0, fmt.Errorf("silero: create state tensor: %s", C.GoString(C.gosfu_error_message(p.api, status)))
	}
	defer C.gosfu_release_value(p.api, stateValue)

	rate := []C.int64_t{defaultRate}
	rateShape := []C.int64_t{1}
	var rateValue *C.OrtValue
	if status := C.gosfu_create_int64_tensor(
		p.api,
		p.memoryInfo,
		(*C.int64_t)(unsafe.Pointer(&rate[0])),
		1,
		(*C.int64_t)(unsafe.Pointer(&rateShape[0])),
		C.size_t(len(rateShape)),
		&rateValue,
	); status != nil {
		defer C.gosfu_release_status(p.api, status)
		return 0, fmt.Errorf("silero: create sample-rate tensor: %s", C.GoString(C.gosfu_error_message(p.api, status)))
	}
	defer C.gosfu_release_value(p.api, rateValue)

	inputs := []*C.OrtValue{inputValue, stateValue, rateValue}
	outputs := []*C.OrtValue{nil, nil}
	inputNames := []*C.char{p.cStrings["input"], p.cStrings["state"], p.cStrings["sr"]}
	outputNames := []*C.char{p.cStrings["output"], p.cStrings["stateN"]}

	if status := C.gosfu_run(
		p.api,
		p.session,
		(**C.char)(unsafe.Pointer(&inputNames[0])),
		(**C.OrtValue)(unsafe.Pointer(&inputs[0])),
		C.size_t(len(inputs)),
		(**C.char)(unsafe.Pointer(&outputNames[0])),
		C.size_t(len(outputNames)),
		(**C.OrtValue)(unsafe.Pointer(&outputs[0])),
	); status != nil {
		defer C.gosfu_release_status(p.api, status)
		return 0, fmt.Errorf("silero: run: %s", C.GoString(C.gosfu_error_message(p.api, status)))
	}
	defer C.gosfu_release_value(p.api, outputs[0])
	defer C.gosfu_release_value(p.api, outputs[1])

	var probData unsafe.Pointer
	if status := C.gosfu_tensor_data(p.api, outputs[0], &probData); status != nil {
		defer C.gosfu_release_status(p.api, status)
		return 0, fmt.Errorf("silero: output tensor data: %s", C.GoString(C.gosfu_error_message(p.api, status)))
	}
	var stateData unsafe.Pointer
	if status := C.gosfu_tensor_data(p.api, outputs[1], &stateData); status != nil {
		defer C.gosfu_release_status(p.api, status)
		return 0, fmt.Errorf("silero: state tensor data: %s", C.GoString(C.gosfu_error_message(p.api, status)))
	}
	C.memcpy(unsafe.Pointer(&p.state[0]), stateData, C.size_t(stateLen*4))

	prob := *(*float32)(probData)
	p.lastProb = prob
	p.currSample += windowSize
	return prob, nil
}

func (p *Provider) step(prob float32) (vad.Event, bool) {
	minSilenceSamples := minSilenceMs * defaultRate / 1000

	switch {
	case prob >= speechThreshold && p.tempEnd != 0:
		p.tempEnd = 0

	case prob >= speechThreshold && !p.triggered:
		p.triggered = true
		return vad.Event{Type: vad.SpeechStart}, true

	case prob < silenceThreshold && p.triggered:
		if p.tempEnd == 0 {
			p.tempEnd = p.currSample
		}
		if p.currSample-p.tempEnd < minSilenceSamples {
			return vad.Event{}, false
		}

		p.tempEnd = 0
		p.triggered = false
		return vad.Event{Type: vad.SpeechEnd}, true
	}
	return vad.Event{}, false
}

func (p *Provider) Reset() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.currSample = 0
	p.triggered = false
	p.tempEnd = 0
	p.lastProb = 0
	p.pending = p.pending[:0]
	for i := range p.state {
		p.state[i] = 0
	}
	for i := range p.ctx {
		p.ctx[i] = 0
	}
}

func (p *Provider) Diagnostics() vad.Diagnostics {
	p.mu.Lock()
	defer p.mu.Unlock()
	return vad.Diagnostics{
		LastProbability:  p.lastProb,
		Speaking:         p.triggered,
		SilentCount:      0,
		PendingSamples:   len(p.pending),
		WindowSize:       windowSize,
		SpeechThreshold:  speechThreshold,
		SilenceThreshold: silenceThreshold,
	}
}

func (p *Provider) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.api == nil {
		return nil
	}
	if p.memoryInfo != nil {
		C.gosfu_release_memory_info(p.api, p.memoryInfo)
		p.memoryInfo = nil
	}
	if p.session != nil {
		C.gosfu_release_session(p.api, p.session)
		p.session = nil
	}
	if p.sessionOpts != nil {
		C.gosfu_release_session_options(p.api, p.sessionOpts)
		p.sessionOpts = nil
	}
	if p.env != nil {
		C.gosfu_release_env(p.api, p.env)
		p.env = nil
	}
	for key, ptr := range p.cStrings {
		C.free(unsafe.Pointer(ptr))
		delete(p.cStrings, key)
	}
	p.api = nil
	return nil
}
