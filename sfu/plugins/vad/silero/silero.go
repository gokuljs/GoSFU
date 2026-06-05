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