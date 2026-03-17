package app

import (
	"path/filepath"
	"testing"

	appai "thawts-client/internal/ai"
	"thawts-client/internal/metadata"
	"thawts-client/internal/storage"
)

func newTestApp(t *testing.T) *App {
	t.Helper()
	store, err := storage.NewSQLiteStorage(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("NewSQLiteStorage: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	a := NewApp(store, appai.NewStubProvider(), metadata.NewStubProvider())
	a.SetTestMode(true)
	return a
}

func TestSaveThoughtReturnsThought(t *testing.T) {
	a := newTestApp(t)

	thought, err := a.SaveThought("hello world")
	if err != nil {
		t.Fatalf("SaveThought: %v", err)
	}
	if thought.ID == 0 {
		t.Error("expected non-zero ID")
	}
	if thought.Content != "hello world" {
		t.Errorf("Content = %q, want %q", thought.Content, "hello world")
	}
	if thought.RawContent != "hello world" {
		t.Errorf("RawContent = %q", thought.RawContent)
	}
}

func TestGetRecentThoughts(t *testing.T) {
	a := newTestApp(t)

	for _, text := range []string{"first", "second", "third"} {
		if _, err := a.SaveThought(text); err != nil {
			t.Fatalf("SaveThought(%q): %v", text, err)
		}
	}

	recent, err := a.GetRecentThoughts(2)
	if err != nil {
		t.Fatalf("GetRecentThoughts: %v", err)
	}
	if len(recent) != 2 {
		t.Errorf("got %d thoughts, want 2", len(recent))
	}
	// Most recent first
	if recent[0].Content != "third" {
		t.Errorf("expected most recent first, got %q", recent[0].Content)
	}
}

func TestSearchThoughts(t *testing.T) {
	a := newTestApp(t)

	a.SaveThought("buy groceries")
	a.SaveThought("call the dentist")
	a.SaveThought("BUY a new keyboard")

	results, err := a.SearchThoughts("buy")
	if err != nil {
		t.Fatalf("SearchThoughts: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("got %d results, want 2", len(results))
	}
}

func TestSearchEmptyQueryReturnsRecent(t *testing.T) {
	a := newTestApp(t)

	a.SaveThought("alpha")
	a.SaveThought("beta")

	results, err := a.SearchThoughts("")
	if err != nil {
		t.Fatalf("SearchThoughts: %v", err)
	}
	if len(results) == 0 {
		t.Error("expected results for empty query")
	}
}

func TestUpdateThoughtPreservesShadowRecord(t *testing.T) {
	a := newTestApp(t)

	saved, _ := a.SaveThought("original")
	updated, err := a.UpdateThought(saved.ID, "edited")
	if err != nil {
		t.Fatalf("UpdateThought: %v", err)
	}
	if updated.Content != "edited" {
		t.Errorf("Content = %q, want edited", updated.Content)
	}
	if updated.RawContent != "original" {
		t.Errorf("RawContent = %q, shadow record must not change", updated.RawContent)
	}
}

func TestDeleteThought(t *testing.T) {
	a := newTestApp(t)

	saved, _ := a.SaveThought("to delete")
	if err := a.DeleteThought(saved.ID); err != nil {
		t.Fatalf("DeleteThought: %v", err)
	}

	_, err := a.GetThought(saved.ID)
	if err == nil {
		t.Error("expected error retrieving deleted thought")
	}
}
