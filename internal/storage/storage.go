// Package storage defines the persistence interface for Thawts.
//
// The default implementation is SQLite (local-only, no CGO). The interface is
// designed so that a future remote/SaaS backend can be swapped in without
// touching the rest of the application.
package storage

import "thawts-client/internal/domain"

// Storage is the persistence contract.
type Storage interface {
	// SaveThought persists a new thought and returns the saved record.
	SaveThought(content string, ctx domain.CaptureContext) (*domain.Thought, error)

	// GetThought retrieves a single thought by ID, including its tags.
	GetThought(id int64) (*domain.Thought, error)

	// UpdateThought edits the visible content of a thought.
	// The raw_content (shadow record) is never modified.
	UpdateThought(id int64, content string) (*domain.Thought, error)

	// DeleteThought removes a thought and all its associated tags.
	DeleteThought(id int64) error

	// SearchThoughts returns thoughts whose content contains query (case-insensitive).
	SearchThoughts(query string, limit int) ([]*domain.Thought, error)

	// GetRecentThoughts returns the most recently captured thoughts.
	GetRecentThoughts(limit int) ([]*domain.Thought, error)

	// AddTag attaches a classification tag to a thought.
	AddTag(thoughtID int64, name, source string, confidence float64) error

	// HideThought marks a thought as hidden (moves it to the "Review Needed" bin).
	HideThought(id int64) error

	// UnhideThought makes a thought visible again and removes any "mishap" tag.
	UnhideThought(id int64) error

	// GetHiddenThoughts returns all thoughts in the "Review Needed" bin.
	GetHiddenThoughts() ([]*domain.Thought, error)

	// SemanticSearch returns thoughts matching query. When vector embeddings are
	// stored, results are ranked by cosine similarity; otherwise falls back to
	// case-insensitive text search.
	SemanticSearch(query string, limit int) ([]*domain.Thought, error)

	// StoreEmbedding persists a dense float32 vector for the given thought.
	// Overwrites any previously stored embedding for that thought.
	StoreEmbedding(thoughtID int64, embedding []float32) error

	// GetEmbeddings returns stored float32 vectors for the given thought IDs.
	// Thoughts without a stored embedding are absent from the returned map.
	GetEmbeddings(ids []int64) (map[int64][]float32, error)

	// MergeThoughts combines multiple thoughts into one new thought, union-merging
	// tags and using the oldest created_at. The originals are soft-deleted.
	MergeThoughts(ids []int64) (*domain.Thought, error)

	// --- Intent management ---

	// SaveIntent persists a new intent derived from a thought.
	SaveIntent(intent domain.Intent) error

	// GetIntent returns a single intent by ID.
	GetIntent(id string) (*domain.Intent, error)

	// GetPendingIntents returns all intents with status "pending".
	GetPendingIntents() ([]domain.Intent, error)

	// ConfirmIntent marks an intent as confirmed.
	ConfirmIntent(id string) error

	// DismissIntent marks an intent as dismissed.
	DismissIntent(id string) error

	// --- Wellbeing / sentiment ---

	// StoreSentiment records a sentiment polarity score for a thought.
	StoreSentiment(thoughtID int64, score float32) error

	// GetSentimentTrend returns wellbeing signals for thoughts captured within
	// the last `days` days, ordered by created_at ascending.
	GetSentimentTrend(days int) ([]domain.WellbeingSignal, error)

	// Close releases underlying resources.
	Close() error
}
