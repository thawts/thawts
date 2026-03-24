//go:build with_onnx && linux && arm64

package onnx

// CGO linker flags for Linux arm64.

/*
#cgo LDFLAGS: -L${SRCDIR}/libs
*/
import "C"
