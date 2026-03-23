//go:build with_onnx && darwin && arm64

package onnx

import _ "embed"

// ortLibBytes holds the ONNX Runtime shared library for macOS arm64 (~30 MB).
// It is extracted to the OS cache dir on first run.
//
//go:embed libs/libonnxruntime.dylib
var ortLibBytes []byte

const ortLibFilename = "libonnxruntime.dylib"
