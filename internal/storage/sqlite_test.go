package storage

import (
	"path/filepath"
	"testing"
	"time"

	"thawts-client/internal/domain"
)

func newTestDB(t *testing.T) *SQLiteStorage {
	t.Helper()
	store, err := NewSQLiteStorage(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("NewSQLiteStorage: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

var emptyCtx = domain.CaptureContext{}

func TestSaveAndGetThought(t *testing.T) {
	store := newTestDB(t)

	ctx := domain.CaptureContext{AppName: "Terminal", WindowTitle: "zsh"}
	got, err := store.SaveThought("hello world", ctx)
	if err != nil {
		t.Fatalf("SaveThought: %v", err)
	}
	if got.ID == 0 {
		t.Fatal("expected non-zero ID")
	}
	if got.Content != "hello world" {
		t.Errorf("Content = %q, want %q", got.Content, "hello world")
	}
	if got.RawContent != "hello world" {
		t.Errorf("RawContent = %q, want %q", got.RawContent, "hello world")
	}
	if got.Context.AppName != "Terminal" {
		t.Errorf("AppName = %q, want Terminal", got.Context.AppName)
	}

	// Fetch by ID
	fetched, err := store.GetThought(got.ID)
	if err != nil {
		t.Fatalf("GetThought: %v", err)
	}
	if fetched.Content != got.Content {
		t.Errorf("fetched content mismatch")
	}
}

func TestUpdateThoughtPreservesRawContent(t *testing.T) {
	store := newTestDB(t)

	saved, _ := store.SaveThought("original text", emptyCtx)
	updated, err := store.UpdateThought(saved.ID, "edited text")
	if err != nil {
		t.Fatalf("UpdateThought: %v", err)
	}

	if updated.Content != "edited text" {
		t.Errorf("Content = %q, want %q", updated.Content, "edited text")
	}
	// Shadow record must remain unchanged.
	if updated.RawContent != "original text" {
		t.Errorf("RawContent = %q, want original text (shadow record must not change)", updated.RawContent)
	}
}

func TestDeleteThought(t *testing.T) {
	store := newTestDB(t)

	saved, _ := store.SaveThought("to be deleted", emptyCtx)
	if err := store.DeleteThought(saved.ID); err != nil {
		t.Fatalf("DeleteThought: %v", err)
	}

	_, err := store.GetThought(saved.ID)
	if err == nil {
		t.Error("expected error after deleting thought, got nil")
	}
}

func TestSearchThoughts(t *testing.T) {
	store := newTestDB(t)

	store.SaveThought("buy milk and eggs", emptyCtx)
	store.SaveThought("meeting at noon", emptyCtx)
	store.SaveThought("MEETING TOMORROW", emptyCtx)

	results, err := store.SearchThoughts("meeting", 10)
	if err != nil {
		t.Fatalf("SearchThoughts: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("got %d results, want 2", len(results))
	}
}

func TestGetRecentThoughts(t *testing.T) {
	store := newTestDB(t)

	for i := 0; i < 7; i++ {
		store.SaveThought("thought", emptyCtx)
		time.Sleep(time.Millisecond) // ensure distinct timestamps
	}

	recent, err := store.GetRecentThoughts(5)
	if err != nil {
		t.Fatalf("GetRecentThoughts: %v", err)
	}
	if len(recent) != 5 {
		t.Errorf("got %d thoughts, want 5", len(recent))
	}
	// Most recent first
	for i := 1; i < len(recent); i++ {
		if recent[i].CreatedAt.After(recent[i-1].CreatedAt) {
			t.Error("results not sorted descending by created_at")
		}
	}
}

func TestAddTagAndRetrieve(t *testing.T) {
	store := newTestDB(t)

	saved, _ := store.SaveThought("buy groceries", emptyCtx)
	if err := store.AddTag(saved.ID, "todo", "regex", 0.9); err != nil {
		t.Fatalf("AddTag: %v", err)
	}

	fetched, _ := store.GetThought(saved.ID)
	if len(fetched.Tags) != 1 {
		t.Fatalf("expected 1 tag, got %d", len(fetched.Tags))
	}
	tag := fetched.Tags[0]
	if tag.Name != "todo" || tag.Source != "regex" {
		t.Errorf("unexpected tag: %+v", tag)
	}
}

func TestDeleteCascadesTags(t *testing.T) {
	store := newTestDB(t)

	saved, _ := store.SaveThought("something", emptyCtx)
	store.AddTag(saved.ID, "idea", "regex", 1.0)
	store.DeleteThought(saved.ID)

	// Tags should be gone (foreign key cascade)
	tagMap, err := store.fetchTagsForIDs([]int64{saved.ID})
	if err != nil {
		t.Fatalf("fetchTagsForIDs: %v", err)
	}
	if len(tagMap[saved.ID]) != 0 {
		t.Error("expected tags to be deleted via cascade")
	}
}

func TestSearchReturnsTagsAttached(t *testing.T) {
	store := newTestDB(t)

	saved, _ := store.SaveThought("dentist appointment next week", emptyCtx)
	store.AddTag(saved.ID, "calendar", "regex", 0.8)

	results, _ := store.SearchThoughts("dentist", 5)
	if len(results) == 0 {
		t.Fatal("no results")
	}
	if len(results[0].Tags) == 0 {
		t.Error("expected tags to be included in search results")
	}
}
