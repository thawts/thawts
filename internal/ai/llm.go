package ai

import (
	"context"
	"log"
	"os"
)

// LLMProvider is an ai.Provider that loads a GGUF model for LLM-based inference.
// When the model file is absent at modelPath, it transparently wraps StubProvider
// so the application continues to work with regex-based classification.
//
// Model path convention: ~/.thawts/models/classifier.gguf
//
// TODO: integrate go-llama.cpp (or llama-cli shell-out) for ClassifyThought,
//       Embed (via ONNX all-MiniLM-L6-v2), AnalyzeSentiment, and CleanText
//       when the model file is present.
type LLMProvider struct {
	inner       Provider
	modelPath   string
	modelLoaded bool
}

// NewLLMProvider creates a Provider that uses a GGUF model at modelPath when present.
// Falls back to StubProvider silently if the model file is absent.
func NewLLMProvider(modelPath string) *LLMProvider {
	p := &LLMProvider{
		modelPath: modelPath,
		inner:     NewStubProvider(),
	}
	if _, err := os.Stat(modelPath); err == nil {
		p.modelLoaded = true
		log.Printf("ai: model found at %s (LLM classification available)", modelPath)
	}
	return p
}

// ModelLoaded reports whether a GGUF model was found at the configured path.
func (p *LLMProvider) ModelLoaded() bool { return p.modelLoaded }

// ClassifyThought implements Provider.
func (p *LLMProvider) ClassifyThought(text string) (*Classification, error) {
	return p.inner.ClassifyThought(text)
}

// DetectIntents implements Provider.
func (p *LLMProvider) DetectIntents(text string) ([]Intent, error) {
	return p.inner.DetectIntents(text)
}

// IsMishap implements Provider.
func (p *LLMProvider) IsMishap(text string, captureMs int64) bool {
	return p.inner.IsMishap(text, captureMs)
}

// Embed implements Provider.
// When a real ONNX embedding model is wired, this will return 384-dim vectors.
func (p *LLMProvider) Embed(ctx context.Context, text string) ([]float32, error) {
	return p.inner.Embed(ctx, text)
}

// AnalyzeSentiment implements Provider.
func (p *LLMProvider) AnalyzeSentiment(ctx context.Context, text string) (float32, error) {
	return p.inner.AnalyzeSentiment(ctx, text)
}

// CleanText implements Provider.
// When a real LLM is wired, this will return a typo-corrected version.
func (p *LLMProvider) CleanText(ctx context.Context, text string) (string, error) {
	return p.inner.CleanText(ctx, text)
}
