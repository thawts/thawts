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
}
