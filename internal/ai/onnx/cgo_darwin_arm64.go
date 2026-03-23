//go:build with_onnx && darwin && arm64

package onnx

// CGO linker flags for macOS arm64.
// Points the linker at libs/libtokenizers.a (fetched by scripts/download_ai_deps.sh).

/*
#cgo LDFLAGS: -L${SRCDIR}/libs
*/
import "C"
