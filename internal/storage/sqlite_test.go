package storage

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/thawts/thawts/internal/domain"
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

func TestHideAndUnhideThought(t *testing.T) {
	store := newTestDB(t)

	saved, _ := store.SaveThought("my password 12345", emptyCtx)
	store.AddTag(saved.ID, "mishap", "ai", 0.9)

	if err := store.HideThought(saved.ID); err != nil {
		t.Fatalf("HideThought: %v", err)
	}

	// Should not appear in normal queries
	recent, _ := store.GetRecentThoughts(10)
	for _, th := range recent {
		if th.ID == saved.ID {
			t.Error("hidden thought appeared in GetRecentThoughts")
		}
	}

	hidden, err := store.GetHiddenThoughts()
	if err != nil {
		t.Fatalf("GetHiddenThoughts: %v", err)
	}
	if len(hidden) != 1 || hidden[0].ID != saved.ID {
		t.Errorf("GetHiddenThoughts returned %d results, want 1", len(hidden))
	}
	if !hidden[0].Hidden {
		t.Error("thought.Hidden should be true")
	}

	// Unhide — should remove mishap tag and restore visibility
	if err := store.UnhideThought(saved.ID); err != nil {
		t.Fatalf("UnhideThought: %v", err)
	}

	restored, _ := store.GetThought(saved.ID)
	if restored.Hidden {
		t.Error("thought.Hidden should be false after UnhideThought")
	}
	for _, tag := range restored.Tags {
		if tag.Name == "mishap" {
			t.Error("mishap tag should have been removed by UnhideThought")
		}
	}

	hidden2, _ := store.GetHiddenThoughts()
	if len(hidden2) != 0 {
		t.Error("GetHiddenThoughts should be empty after UnhideThought")
	}
}

func TestSearchExcludesHidden(t *testing.T) {
	store := newTestDB(t)

	store.SaveThought("visible thought", emptyCtx)
	hidden, _ := store.SaveThought("hidden thought", emptyCtx)
	store.HideThought(hidden.ID)

	results, _ := store.SearchThoughts("thought", 10)
	for _, r := range results {
		if r.ID == hidden.ID {
			t.Error("SearchThoughts returned a hidden thought")
		}
	}
}

func TestStoreAndGetEmbeddings(t *testing.T) {
	store := newTestDB(t)

	t1, _ := store.SaveThought("machine learning is fascinating", emptyCtx)
	t2, _ := store.SaveThought("deep neural networks", emptyCtx)

	vec1 := []float32{0.1, 0.2, 0.3, 0.4}
	vec2 := []float32{0.5, 0.6, 0.7, 0.8}

	if err := store.StoreEmbedding(t1.ID, vec1); err != nil {
		t.Fatalf("StoreEmbedding t1: %v", err)
	}
	if err := store.StoreEmbedding(t2.ID, vec2); err != nil {
		t.Fatalf("StoreEmbedding t2: %v", err)
	}

	// Overwrite — should not error
	if err := store.StoreEmbedding(t1.ID, vec1); err != nil {
		t.Fatalf("StoreEmbedding overwrite: %v", err)
	}

	embs, err := store.GetEmbeddings([]int64{t1.ID, t2.ID})
	if err != nil {
		t.Fatalf("GetEmbeddings: %v", err)
	}
	if len(embs) != 2 {
		t.Fatalf("expected 2 embeddings, got %d", len(embs))
	}
	for i, v := range vec1 {
		if embs[t1.ID][i] != v {
			t.Errorf("embedding[%d] = %v, want %v", i, embs[t1.ID][i], v)
		}
	}
}

func TestSemanticSearchFallsBackToText(t *testing.T) {
	store := newTestDB(t)

	store.SaveThought("meeting tomorrow at noon", emptyCtx)
	store.SaveThought("buy groceries today", emptyCtx)

	// No embeddings stored — should fall back to text LIKE search.
	results, err := store.SemanticSearch("meeting", 10)
	if err != nil {
		t.Fatalf("SemanticSearch: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
	if results[0].Content != "meeting tomorrow at noon" {
		t.Errorf("unexpected result: %q", results[0].Content)
	}
}

func TestGetEmbeddingsEmpty(t *testing.T) {
	store := newTestDB(t)
	embs, err := store.GetEmbeddings([]int64{})
	if err != nil {
		t.Fatalf("GetEmbeddings empty: %v", err)
	}
	if embs != nil {
		t.Errorf("expected nil for empty input, got %v", embs)
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

// --- Intent tests ---

func TestSaveAndGetPendingIntents(t *testing.T) {
	store := newTestDB(t)

	thought, _ := store.SaveThought("meeting with team tomorrow at 10am", emptyCtx)

	intent := domain.Intent{
		ID:        "test-intent-1",
		ThoughtID: thought.ID,
		Type:      "calendar",
		Title:     "meeting with team",
		Status:    "pending",
		CreatedAt: time.Now().UTC(),
	}
	if err := store.SaveIntent(intent); err != nil {
		t.Fatalf("SaveIntent: %v", err)
	}

	pending, err := store.GetPendingIntents()
	if err != nil {
		t.Fatalf("GetPendingIntents: %v", err)
	}
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending intent, got %d", len(pending))
	}
	if pending[0].ID != "test-intent-1" {
		t.Errorf("intent ID = %q, want test-intent-1", pending[0].ID)
	}
	if pending[0].Type != "calendar" {
		t.Errorf("intent type = %q, want calendar", pending[0].Type)
	}
}

func TestGetIntent(t *testing.T) {
	store := newTestDB(t)

	thought, _ := store.SaveThought("remind me to call doctor", emptyCtx)
	intent := domain.Intent{
		ID:        "test-intent-2",
		ThoughtID: thought.ID,
		Type:      "reminder",
		Title:     "call doctor",
		Status:    "pending",
		CreatedAt: time.Now().UTC(),
	}
	store.SaveIntent(intent)

	got, err := store.GetIntent("test-intent-2")
	if err != nil {
		t.Fatalf("GetIntent: %v", err)
	}
	if got.Title != "call doctor" {
		t.Errorf("Title = %q, want call doctor", got.Title)
	}
}

func TestConfirmAndDismissIntent(t *testing.T) {
	store := newTestDB(t)

	thought, _ := store.SaveThought("buy groceries", emptyCtx)

	intentA := domain.Intent{ID: "ia", ThoughtID: thought.ID, Type: "task", Title: "buy groceries", Status: "pending", CreatedAt: time.Now().UTC()}
	intentB := domain.Intent{ID: "ib", ThoughtID: thought.ID, Type: "task", Title: "meal prep", Status: "pending", CreatedAt: time.Now().UTC()}
	store.SaveIntent(intentA)
	store.SaveIntent(intentB)

	if err := store.ConfirmIntent("ia"); err != nil {
		t.Fatalf("ConfirmIntent: %v", err)
	}
	if err := store.DismissIntent("ib"); err != nil {
		t.Fatalf("DismissIntent: %v", err)
	}

	pending, _ := store.GetPendingIntents()
	if len(pending) != 0 {
		t.Errorf("expected 0 pending intents after confirm+dismiss, got %d", len(pending))
	}

	confirmed, _ := store.GetIntent("ia")
	if confirmed.Status != "confirmed" {
		t.Errorf("status = %q, want confirmed", confirmed.Status)
	}
}

func TestIntentCascadeDeletesWithThought(t *testing.T) {
	store := newTestDB(t)

	thought, _ := store.SaveThought("lunch with Alice tomorrow", emptyCtx)
	store.SaveIntent(domain.Intent{ID: "ic", ThoughtID: thought.ID, Type: "calendar", Title: "lunch", Status: "pending", CreatedAt: time.Now().UTC()})

	store.DeleteThought(thought.ID)

	_, err := store.GetIntent("ic")
	if err == nil {
		t.Error("expected error retrieving intent after parent thought deleted")
	}
}

// --- Wellbeing signal tests ---

func TestStoreSentimentAndGetTrend(t *testing.T) {
	store := newTestDB(t)

	t1, _ := store.SaveThought("feeling really tired and overwhelmed", emptyCtx)
	t2, _ := store.SaveThought("things are awful today", emptyCtx)

	if err := store.StoreSentiment(t1.ID, -0.6); err != nil {
		t.Fatalf("StoreSentiment t1: %v", err)
	}
	if err := store.StoreSentiment(t2.ID, -0.8); err != nil {
		t.Fatalf("StoreSentiment t2: %v", err)
	}

	signals, err := store.GetSentimentTrend(7)
	if err != nil {
		t.Fatalf("GetSentimentTrend: %v", err)
	}
	if len(signals) != 2 {
		t.Fatalf("expected 2 signals, got %d", len(signals))
	}
}

func TestGetSentimentTrendEmptyWindow(t *testing.T) {
	store := newTestDB(t)

	signals, err := store.GetSentimentTrend(7)
	if err != nil {
		t.Fatalf("GetSentimentTrend: %v", err)
	}
	if len(signals) != 0 {
		t.Errorf("expected 0 signals on empty DB, got %d", len(signals))
	}
}

// --- MergeThoughts tests ---

func TestMergeThoughts(t *testing.T) {
	store := newTestDB(t)

	t1, _ := store.SaveThought("first fragment", emptyCtx)
	t2, _ := store.SaveThought("second fragment", emptyCtx)
	store.AddTag(t1.ID, "idea", "regex", 0.9)
	store.AddTag(t2.ID, "todo", "regex", 0.8)

	merged, err := store.MergeThoughts([]int64{t1.ID, t2.ID})
	if err != nil {
		t.Fatalf("MergeThoughts: %v", err)
	}
	if merged.ID == 0 {
		t.Fatal("merged thought has zero ID")
	}

	// Content should contain both fragments
	if !containsStr(merged.Content, "first fragment") || !containsStr(merged.Content, "second fragment") {
		t.Errorf("merged content missing fragments: %q", merged.Content)
	}

	// Tags should be union-merged
	tagNames := map[string]bool{}
	for _, tag := range merged.Tags {
		tagNames[tag.Name] = true
	}
	if !tagNames["idea"] || !tagNames["todo"] {
		t.Errorf("merged tags missing: %v", tagNames)
	}

	// Meta should contain merged_from
	if merged.Meta == nil {
		t.Fatal("merged thought has no meta")
	}

	// Originals should be hidden
	hidden, _ := store.GetHiddenThoughts()
	hiddenIDs := map[int64]bool{}
	for _, h := range hidden {
		hiddenIDs[h.ID] = true
	}
	if !hiddenIDs[t1.ID] || !hiddenIDs[t2.ID] {
		t.Error("original thoughts should be hidden after merge")
	}
}

func TestMergeThoughtsRequiresAtLeastTwo(t *testing.T) {
	store := newTestDB(t)
	t1, _ := store.SaveThought("only one", emptyCtx)
	_, err := store.MergeThoughts([]int64{t1.ID})
	if err == nil {
		t.Error("expected error when merging fewer than 2 thoughts")
	}
}

func containsStr(s, sub string) bool {
	return strings.Contains(s, sub)
}
