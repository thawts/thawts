package domain

import "time"

// CaptureContext holds metadata about the environment when a thought was captured.
// All fields are populated by the metadata.Provider; defaults to empty strings.
type CaptureContext struct {
	WindowTitle string `json:"window_title"`
	AppName     string `json:"app_name"`
	URL         string `json:"url"`
}

// Tag is a classification label attached to a thought.
type Tag struct {
	ID         int64     `json:"id"`
	ThoughtID  int64     `json:"thought_id"`
	Name       string    `json:"name"`
	Source     string    `json:"source"`     // "regex", "ai", "user"
	Confidence float64   `json:"confidence"` // 0.0–1.0
	CreatedAt  time.Time `json:"created_at"`
}

// Thought is the central domain entity.
//
// RawContent is the original text captured from the user — it is immutable (the
// "shadow record"). Content may be edited later in Review Mode; RawContent never is.
// Hidden thoughts are filtered from normal views and land in the "Review Needed" bin.
// Meta holds arbitrary structured metadata (e.g. merged_from for merged thoughts).
type Thought struct {
	ID         int64          `json:"id"`
	Content    string         `json:"content"`
	RawContent string         `json:"raw_content"`
	Context    CaptureContext `json:"context"`
	Tags       []Tag          `json:"tags"`
	Hidden     bool           `json:"hidden"`
	Meta       map[string]any `json:"meta,omitempty"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
}

// Intent represents an actionable item extracted from a thought and awaiting
// user confirmation before being created in a native calendar or reminder app.
type Intent struct {
	ID        string     `json:"id"`
	ThoughtID int64      `json:"thought_id"`
	Type      string     `json:"type"`   // "calendar" | "task" | "reminder"
	Title     string     `json:"title"`
	Date      *time.Time `json:"date,omitempty"`
	Status    string     `json:"status"` // "pending" | "confirmed" | "dismissed"
	CreatedAt time.Time  `json:"created_at"`
}

// WellbeingSignal records a sentiment polarity score for a thought.
// Score is in the range [-1.0, +1.0]: positive values indicate positive sentiment.
type WellbeingSignal struct {
	ThoughtID int64     `json:"thought_id"`
	Score     float32   `json:"score"`
	CreatedAt time.Time `json:"created_at"`
}
