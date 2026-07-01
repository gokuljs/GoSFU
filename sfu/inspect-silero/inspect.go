package main

import (
	"fmt"
	"os"
	"runtime"

	ort "github.com/yalue/onnxruntime_go"
)

func main() {
	libPath := os.Getenv("ONNXRUNTIME_LIB_PATH")
	if libPath == "" {
		switch runtime.GOOS {
		case "darwin":
			if runtime.GOARCH == "arm64" {
				libPath = "/opt/homebrew/lib/libonnxruntime.dylib"
			} else {
				libPath = "/usr/local/lib/libonnxruntime.dylib"
			}
		default:
			libPath = "/usr/local/lib/libonnxruntime.so"
		}
	}

	modelPath := os.Getenv("SILERO_MODEL_PATH")
	if modelPath == "" {
		modelPath = "assets/models/silero_vad.onnx"
	}

	ort.SetSharedLibraryPath(libPath)
	if err := ort.InitializeEnvironment(); err != nil {
		panic(err)
	}
	defer ort.DestroyEnvironment()

	in, out, err := ort.GetInputOutputInfo(modelPath)
	if err != nil {
		panic(err)
	}
	for _, i := range in {
		fmt.Printf("INPUT  %s shape=%v type=%v\n", i.Name, i.Dimensions, i.DataType)
	}
	for _, o := range out {
		fmt.Printf("OUTPUT %s shape=%v type=%v\n", o.Name, o.Dimensions, o.DataType)
	}
}
