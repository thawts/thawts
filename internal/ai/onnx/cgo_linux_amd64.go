//go:build with_onnx && linux && amd64

package onnx

// CGO linker flags for Linux amd64.

/*
#cgo LDFLAGS: -L${SRCDIR}/libs
*/
import "C"
