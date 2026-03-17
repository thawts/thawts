package app

import (
	"path/filepath"
	"testing"

	appai "thawts-client/internal/ai"
	"thawts-client/internal/domain"
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

// --- cosineSimilarity ---

func TestCosineSimilarity(t *testing.T) {
	cases := []struct {
		name string
		a, b []float32
		want float32
	}{
		{"identical", []float32{1, 0, 0}, []float32{1, 0, 0}, 1.0},
		{"orthogonal", []float32{1, 0}, []float32{0, 1}, 0.0},
		{"opposite", []float32{1, 0}, []float32{-1, 0}, -1.0},
		{"zero length a", nil, []float32{1}, 0.0},
		{"dim mismatch", []float32{1, 2}, []float32{1}, 0.0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := cosineSimilarity(tc.a, tc.b)
			if diff := got - tc.want; diff < -0.001 || diff > 0.001 {
				t.Errorf("cosineSimilarity = %v, want %v", got, tc.want)
			}
		})
	}
}

// --- classifyAsync ---

func TestClassifyAsyncMishapHidesThought(t *testing.T) {
	a := newTestApp(t)

	saved, err := a.store.SaveThought("aX9#kP!2@mZ&qR5s", domain.CaptureContext{})
	if err != nil {
		t.Fatalf("SaveThought: %v", err)
	}

	a.classifyAsync(saved.ID, saved.Content)

	hidden, err := a.store.GetHiddenThoughts()
	if err != nil {
		t.Fatalf("GetHiddenThoughts: %v", err)
	}
	found := false
	for _, th := range hidden {
		if th.ID == saved.ID {
			found = true
		}
	}
	if !found {
		t.Error("mishap thought was not hidden by classifyAsync")
	}

	th, _ := a.store.GetThought(saved.ID)
	hasMishapTag := false
	for _, tag := range th.Tags {
		if tag.Name == "mishap" {
			hasMishapTag = true
		}
	}
	if !hasMishapTag {
		t.Error("mishap tag was not attached by classifyAsync")
	}
}

func TestClassifyAsyncTagsNormalThought(t *testing.T) {
	a := newTestApp(t)

	saved, err := a.store.SaveThought("buy milk tomorrow", domain.CaptureContext{})
	if err != nil {
		t.Fatalf("SaveThought: %v", err)
	}

	a.classifyAsync(saved.ID, saved.Content)

	th, err := a.store.GetThought(saved.ID)
	if err != nil {
		t.Fatalf("GetThought: %v", err)
	}
	if len(th.Tags) == 0 {
		t.Error("expected tags to be attached for a classified thought")
	}
	if th.Hidden {
		t.Error("normal thought should not be hidden")
	}
}

// --- SemanticSearch fallback ---

func TestSemanticSearchFallsBackToText(t *testing.T) {
	a := newTestApp(t)

	a.SaveThought("machine learning conference next week")
	a.SaveThought("buy groceries today")

	results, err := a.SemanticSearch("machine learning")
	if err != nil {
		t.Fatalf("SemanticSearch: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected at least one result")
	}
	if results[0].Content != "machine learning conference next week" {
		t.Errorf("unexpected top result: %q", results[0].Content)
	}
}
