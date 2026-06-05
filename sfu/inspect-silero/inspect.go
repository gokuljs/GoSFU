package main

import (
	"fmt"

	ort "github.com/yalue/onnxruntime_go"
)

func main() {
	ort.SetSharedLibraryPath("/opt/homebrew/lib/libonnxruntime.dylib")
	ort.GetInputOutputInfo("assets/models/silero_vad.onnx")
	if err := ort.InitializeEnvironment(); err != nil {
		panic(err)
	}
	defer ort.DestroyEnvironment()

	in, out, err := ort.GetInputOutputInfo("assets/models/silero_vad.onnx")
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
