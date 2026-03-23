//go:build with_onnx && linux && amd64

package onnx

import _ "embed"

//go:embed libs/libonnxruntime.so
var ortLibBytes []byte

const ortLibFilename = "libonnxruntime.so"
