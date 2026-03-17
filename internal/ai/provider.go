// Package ai defines the AI/ML contract for Thawts.
//
// The default implementation is a local regex-based stub that requires no
// external dependencies. This can be replaced with:
//   - A local LLM via GGUF/ONNX bindings (e.g. go-llama.cpp)
//   - An external API (OpenAI-compatible, Anthropic, etc.)
//   - A hosted Thawts SaaS backend
//
// All swaps require only changing the Provider passed to app.NewApp.
package ai

import "context"

// Classification is the result of analysing a thought's content.
type Classification struct {
	Tags []ClassifiedTag
}

// ClassifiedTag is a tag produced by classification.
type ClassifiedTag struct {
	Name       string
	Confidence float64 // 0.0–1.0
}

// Intent represents an actionable item detected in a thought.
type Intent struct {
	Type        string // "calendar", "task", "reminder"
	Description string
	Raw         string // the original matching text
}

// Provider is the AI interface consumed by the application.
type Provider interface {
	// ClassifyThought analyses text and returns zero or more classification tags.
	ClassifyThought(text string) (*Classification, error)

	// DetectIntents scans text for actionable items (calendar events, tasks, reminders).
	DetectIntents(text string) ([]Intent, error)

	// IsMishap reports whether text looks like an accidental capture (password,
	// code snippet, gibberish, or large paste). captureMs is the elapsed time
	// between focus and Enter in milliseconds; 0 means unknown.
	IsMishap(text string, captureMs int64) bool

	// Embed produces a dense vector representation of text (384-dim float32).
	// Returns nil when the provider does not support embeddings (e.g. stub).
	// Callers must treat a nil return as "no embedding available".
	Embed(ctx context.Context, text string) ([]float32, error)

	// AnalyzeSentiment returns a polarity score in [-1.0, +1.0].
	// Positive values indicate positive sentiment; negative indicates negative.
	AnalyzeSentiment(ctx context.Context, text string) (float32, error)

	// CleanText returns a typo-corrected, punctuated version of text for
	// reading clarity. The original is never modified; this is display-only.
	// Returns the original text unchanged when no LLM is available.
	CleanText(ctx context.Context, text string) (string, error)
}
