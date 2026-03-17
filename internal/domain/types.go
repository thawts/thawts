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
type Thought struct {
	ID         int64          `json:"id"`
	Content    string         `json:"content"`
	RawContent string         `json:"raw_content"`
	Context    CaptureContext `json:"context"`
	Tags       []Tag          `json:"tags"`
	Hidden     bool           `json:"hidden"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
}
