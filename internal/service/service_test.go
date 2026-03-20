package service

import (
	"path/filepath"
	"testing"
	"time"

	appai "thawts-client/internal/ai"
	"thawts-client/internal/domain"
	"thawts-client/internal/metadata"
	"thawts-client/internal/storage"
)

func newTestService(t *testing.T) *Service {
	t.Helper()
	store, err := storage.NewSQLiteStorage(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("NewSQLiteStorage: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return New(store, appai.NewStubProvider(), metadata.NewStubProvider(), &NoopNotifier{})
}

// --- SaveThought ---

func TestSaveThoughtReturnsThought(t *testing.T) {
	s := newTestService(t)

	thought, err := s.SaveThought("hello world")
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
		t.Errorf("RawContent = %q, want %q", thought.RawContent, "hello world")
	}
}

// --- GetRecentThoughts ---

func TestGetRecentThoughts_orderedNewestFirst(t *testing.T) {
	s := newTestService(t)

	for _, text := range []string{"first", "second", "third"} {
		if _, err := s.SaveThought(text); err != nil {
			t.Fatalf("SaveThought(%q): %v", text, err)
		}
	}

	recent, err := s.GetRecentThoughts(2)
	if err != nil {
		t.Fatalf("GetRecentThoughts: %v", err)
	}
	if len(recent) != 2 {
		t.Errorf("got %d thoughts, want 2", len(recent))
	}
	if recent[0].Content != "third" {
		t.Errorf("expected most recent first, got %q", recent[0].Content)
	}
}

func TestGetRecentThoughts_respectsLimit(t *testing.T) {
	s := newTestService(t)

	for range 5 {
		s.SaveThought("thought")
	}
	recent, err := s.GetRecentThoughts(3)
	if err != nil {
		t.Fatalf("GetRecentThoughts: %v", err)
	}
	if len(recent) != 3 {
		t.Errorf("got %d thoughts, want 3", len(recent))
	}
}

// --- SearchThoughts ---

func TestSearchThoughts_caseInsensitiveSubstring(t *testing.T) {
	s := newTestService(t)

	s.SaveThought("buy groceries")
	s.SaveThought("call the dentist")
	s.SaveThought("BUY a new keyboard")

	results, err := s.SearchThoughts("buy")
	if err != nil {
		t.Fatalf("SearchThoughts: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("got %d results, want 2", len(results))
	}
}

func TestSearchThoughts_noMatch(t *testing.T) {
	s := newTestService(t)

	s.SaveThought("buy groceries")
	results, err := s.SearchThoughts("zzznomatch")
	if err != nil {
		t.Fatalf("SearchThoughts: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("got %d results, want 0", len(results))
	}
}

func TestSearchThoughts_emptyQueryReturnsRecent(t *testing.T) {
	s := newTestService(t)

	s.SaveThought("alpha")
	s.SaveThought("beta")

	results, err := s.SearchThoughts("")
	if err != nil {
		t.Fatalf("SearchThoughts: %v", err)
	}
	if len(results) == 0 {
		t.Error("expected results for empty query")
	}
}

// --- UpdateThought ---

func TestUpdateThought_changesContent(t *testing.T) {
	s := newTestService(t)

	saved, _ := s.SaveThought("original")
	updated, err := s.UpdateThought(saved.ID, "edited")
	if err != nil {
		t.Fatalf("UpdateThought: %v", err)
	}
	if updated.Content != "edited" {
		t.Errorf("Content = %q, want edited", updated.Content)
	}
}

func TestUpdateThought_rawContentIsImmutable(t *testing.T) {
	s := newTestService(t)

	saved, _ := s.SaveThought("original")
	updated, err := s.UpdateThought(saved.ID, "edited")
	if err != nil {
		t.Fatalf("UpdateThought: %v", err)
	}
	if updated.RawContent != "original" {
		t.Errorf("RawContent = %q, shadow record must not change", updated.RawContent)
	}
}

// --- DeleteThought ---

func TestDeleteThought_removesFromRecent(t *testing.T) {
	s := newTestService(t)

	saved, _ := s.SaveThought("to delete")
	if err := s.DeleteThought(saved.ID); err != nil {
		t.Fatalf("DeleteThought: %v", err)
	}

	_, err := s.GetThought(saved.ID)
	if err == nil {
		t.Error("expected error retrieving deleted thought")
	}
}

// --- GetThought ---

func TestGetThought_notFoundReturnsError(t *testing.T) {
	s := newTestService(t)
	_, err := s.GetThought(99999)
	if err == nil {
		t.Error("expected error for non-existent ID")
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

func TestClassifyAsync_mishapHidesThought(t *testing.T) {
	s := newTestService(t)

	saved, err := s.store.SaveThought("aX9#kP!2@mZ&qR5s", domain.CaptureContext{})
	if err != nil {
		t.Fatalf("SaveThought: %v", err)
	}

	s.classifyAsync(saved.ID, saved.Content)

	hidden, err := s.store.GetHiddenThoughts()
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

	th, _ := s.store.GetThought(saved.ID)
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

func TestClassifyAsync_tagsNormalThought(t *testing.T) {
	s := newTestService(t)

	saved, err := s.store.SaveThought("buy milk tomorrow", domain.CaptureContext{})
	if err != nil {
		t.Fatalf("SaveThought: %v", err)
	}

	s.classifyAsync(saved.ID, saved.Content)

	th, err := s.store.GetThought(saved.ID)
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

func TestClassifyAsync_emitsThoughtClassifiedEvent(t *testing.T) {
	rec := &RecordingNotifier{}
	store, err := storage.NewSQLiteStorage(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("NewSQLiteStorage: %v", err)
	}
	defer store.Close()
	s := New(store, appai.NewStubProvider(), metadata.NewStubProvider(), rec)

	saved, _ := s.store.SaveThought("feeling great today", domain.CaptureContext{})
	s.classifyAsync(saved.ID, saved.Content)

	if !rec.HasEvent("thought:classified") {
		t.Error("expected thought:classified event to be emitted")
	}
}

func TestClassifyAsync_mishapEmitsMishapsChanged(t *testing.T) {
	rec := &RecordingNotifier{}
	store, err := storage.NewSQLiteStorage(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("NewSQLiteStorage: %v", err)
	}
	defer store.Close()
	s := New(store, appai.NewStubProvider(), metadata.NewStubProvider(), rec)

	saved, _ := s.store.SaveThought("aX9#kP!2@mZ&qR5s", domain.CaptureContext{})
	s.classifyAsync(saved.ID, saved.Content)

	if !rec.HasEvent("mishaps:changed") {
		t.Error("expected mishaps:changed event for mishap content")
	}
}

// --- SemanticSearch ---

func TestSemanticSearch_fallsBackToTextWhenNoEmbeddings(t *testing.T) {
	s := newTestService(t)

	s.SaveThought("machine learning conference next week")
	s.SaveThought("buy groceries today")

	results, err := s.SemanticSearch("machine learning")
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

// --- Hidden thoughts ---

func TestGetHiddenThoughts_onlyReturnsFlagged(t *testing.T) {
	s := newTestService(t)

	visible, _ := s.SaveThought("normal thought")
	hidden, _ := s.SaveThought("mishap thought")
	s.store.HideThought(hidden.ID)

	hiddenList, err := s.GetHiddenThoughts()
	if err != nil {
		t.Fatalf("GetHiddenThoughts: %v", err)
	}
	for _, th := range hiddenList {
		if th.ID == visible.ID {
			t.Error("visible thought appeared in hidden list")
		}
	}
	found := false
	for _, th := range hiddenList {
		if th.ID == hidden.ID {
			found = true
		}
	}
	if !found {
		t.Error("hidden thought not returned by GetHiddenThoughts")
	}
}

func TestUnhideThought_restoresToRecent(t *testing.T) {
	s := newTestService(t)

	saved, _ := s.SaveThought("oops")
	s.store.HideThought(saved.ID)

	if err := s.UnhideThought(saved.ID); err != nil {
		t.Fatalf("UnhideThought: %v", err)
	}

	recent, _ := s.GetRecentThoughts(10)
	found := false
	for _, th := range recent {
		if th.ID == saved.ID {
			found = true
		}
	}
	if !found {
		t.Error("unhidden thought not visible in recent thoughts")
	}
}

// --- MergeThoughts ---

func TestMergeThoughts_combinesContent(t *testing.T) {
	s := newTestService(t)

	t1, _ := s.SaveThought("first idea fragment")
	t2, _ := s.SaveThought("second idea fragment")

	merged, err := s.MergeThoughts([]int64{t1.ID, t2.ID})
	if err != nil {
		t.Fatalf("MergeThoughts: %v", err)
	}
	if merged.ID == 0 {
		t.Fatal("expected non-zero merged ID")
	}
}

func TestMergeThoughts_hidesOriginals(t *testing.T) {
	s := newTestService(t)

	t1, _ := s.SaveThought("first idea fragment")
	t2, _ := s.SaveThought("second idea fragment")

	s.MergeThoughts([]int64{t1.ID, t2.ID})

	recent, _ := s.GetRecentThoughts(20)
	for _, r := range recent {
		if r.ID == t1.ID || r.ID == t2.ID {
			t.Error("original thought still visible after merge")
		}
	}
}

// --- Intent actions ---

func TestGetPendingIntents_emptyByDefault(t *testing.T) {
	s := newTestService(t)
	intents, err := s.GetPendingIntents()
	if err != nil {
		t.Fatalf("GetPendingIntents: %v", err)
	}
	if len(intents) != 0 {
		t.Errorf("expected 0 intents, got %d", len(intents))
	}
}

func TestClassifyAsync_storesIntents(t *testing.T) {
	s := newTestService(t)

	saved, _ := s.store.SaveThought("meeting with Alice at 3pm", domain.CaptureContext{})
	s.classifyAsync(saved.ID, saved.Content)

	intents, err := s.GetPendingIntents()
	if err != nil {
		t.Fatalf("GetPendingIntents: %v", err)
	}
	if len(intents) == 0 {
		t.Error("expected at least one pending intent for calendar-like content")
	}
}

func TestConfirmIntent_removesFromPending(t *testing.T) {
	s := newTestService(t)

	saved, _ := s.store.SaveThought("meeting with Alice at 3pm", domain.CaptureContext{})
	s.classifyAsync(saved.ID, saved.Content)

	intents, _ := s.GetPendingIntents()
	if len(intents) == 0 {
		t.Skip("no intents generated by stub for this input")
	}

	id := intents[0].ID
	if err := s.ConfirmIntent(id); err != nil {
		t.Fatalf("ConfirmIntent: %v", err)
	}

	remaining, _ := s.GetPendingIntents()
	for _, i := range remaining {
		if i.ID == id {
			t.Error("confirmed intent still appears in pending list")
		}
	}
}

func TestDismissIntent_removesFromPending(t *testing.T) {
	s := newTestService(t)

	saved, _ := s.store.SaveThought("remind me to call doctor", domain.CaptureContext{})
	s.classifyAsync(saved.ID, saved.Content)

	intents, _ := s.GetPendingIntents()
	if len(intents) == 0 {
		t.Skip("no intents generated by stub for this input")
	}

	id := intents[0].ID
	if err := s.DismissIntent(id); err != nil {
		t.Fatalf("DismissIntent: %v", err)
	}

	remaining, _ := s.GetPendingIntents()
	for _, i := range remaining {
		if i.ID == id {
			t.Error("dismissed intent still appears in pending list")
		}
	}
}

// --- Wellbeing ---

func TestGetSentimentTrend_zeroOnEmptyDB(t *testing.T) {
	s := newTestService(t)
	avg, err := s.GetSentimentTrend(7)
	if err != nil {
		t.Fatalf("GetSentimentTrend: %v", err)
	}
	if avg != 0 {
		t.Errorf("expected 0 on empty DB, got %v", avg)
	}
}

func TestClassifyAsync_storesSentiment(t *testing.T) {
	s := newTestService(t)

	saved, _ := s.store.SaveThought("feeling happy and excited today", domain.CaptureContext{})
	s.classifyAsync(saved.ID, saved.Content)

	avg, err := s.GetSentimentTrend(7)
	if err != nil {
		t.Fatalf("GetSentimentTrend: %v", err)
	}
	if avg == 0 {
		t.Error("expected non-zero sentiment average after classifyAsync")
	}
}

func TestGetSentimentTrend_returnsSignalsInWindow(t *testing.T) {
	s := newTestService(t)

	// Store a signal for a thought
	saved, _ := s.store.SaveThought("test", domain.CaptureContext{})
	s.store.StoreSentiment(saved.ID, 0.5)

	avg, err := s.GetSentimentTrend(7)
	if err != nil {
		t.Fatalf("GetSentimentTrend: %v", err)
	}
	if avg == 0 {
		t.Error("expected non-zero trend when signals exist within window")
	}
}

// --- CleanText ---

func TestCleanText_returnsNonEmpty(t *testing.T) {
	s := newTestService(t)

	saved, _ := s.SaveThought("this is a test thot")
	cleaned, err := s.CleanText(saved.ID)
	if err != nil {
		t.Fatalf("CleanText: %v", err)
	}
	if cleaned == "" {
		t.Error("expected non-empty cleaned text")
	}
}

func TestCleanText_errorForMissingThought(t *testing.T) {
	s := newTestService(t)
	_, err := s.CleanText(99999)
	if err == nil {
		t.Error("expected error for non-existent thought ID")
	}
}

// --- PrepareCapture ---

func TestPrepareCapture_doesNotBlock(t *testing.T) {
	s := newTestService(t)

	done := make(chan struct{})
	go func() {
		s.PrepareCapture()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Error("PrepareCapture blocked for more than 500ms")
	}
}
