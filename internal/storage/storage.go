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

	// Close releases underlying resources.
	Close() error
}
