//go:build !with_onnx

// Package onnx exposes a NewProvider function that returns the stub provider
// when the binary is built without the with_onnx build tag.
//
// To enable real ONNX inference:
//  1. Run: bash scripts/download_ai_deps.sh
//  2. Build: go build -tags with_onnx ./...
package onnx

import "github.com/thawts/thawts/internal/ai"

// NewProvider returns the regex-based stub provider.
// Real ONNX inference requires building with -tags with_onnx.
func NewProvider() ai.Provider {
	return ai.NewStubProvider()
}
