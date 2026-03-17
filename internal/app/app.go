// Package app contains the Wails-bound application struct.
package app

import (
	"context"
	"log"
	"math"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"thawts-client/internal/ai"
	"thawts-client/internal/domain"
	"thawts-client/internal/metadata"
	"thawts-client/internal/storage"
)

// cosineSimilarity returns the cosine similarity in [−1, +1] between two vectors.
// Returns 0 for zero-length or dimension-mismatched vectors.
func cosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}
	denom := math.Sqrt(normA) * math.Sqrt(normB)
	if denom == 0 {
		return 0
	}
	return float32(dot / denom)
}

const (
	captureWidth  = 800
	captureHeight = 60
	reviewWidth   = 1200
	reviewHeight  = 750
)

// App is the Wails application struct. All exported methods are callable from
// the frontend via the generated JS bindings.
type App struct {
	ctx      context.Context
	store    storage.Storage
	ai       ai.Provider
	meta     metadata.Provider
	testMode bool // disables runtime calls during unit tests

	// captureCtx holds the window context captured just before the capture
	// window is shown, so SaveThought can attribute thoughts to the app
	// that was active when the hotkey was triggered (not Thawts itself).
	captureCtxMu sync.Mutex
	captureCtx   domain.CaptureContext
}

// NewApp constructs the App with its dependencies injected.
func NewApp(store storage.Storage, aiProvider ai.Provider, metaProvider metadata.Provider) *App {
	return &App{
		store: store,
		ai:    aiProvider,
		meta:  metaProvider,
	}
}

// Startup is called by Wails once the application context is ready.
func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
}

// Context returns the Wails runtime context (used by main.go for menu callbacks).
func (a *App) Context() context.Context {
	return a.ctx
}

// Quit shuts down the application.
func (a *App) Quit() {
	if !a.testMode {
		runtime.Quit(a.ctx)
	}
}

// SetTestMode disables Wails runtime calls so methods can be tested without a
// running Wails instance.
func (a *App) SetTestMode(v bool) {
	a.testMode = v
}

// --- Capture ---

// SaveThought persists a thought, then classifies it asynchronously.
// Returns the saved thought immediately (before classification completes).
func (a *App) SaveThought(content string) (*domain.Thought, error) {
	a.captureCtxMu.Lock()
	ctx := a.captureCtx
	a.captureCtxMu.Unlock()

	thought, err := a.store.SaveThought(content, ctx)
	if err != nil {
		return nil, err
	}

	// Classify in the background so the UI is never blocked.
	go a.classifyAsync(thought.ID, content)

	return thought, nil
}

// collectMetadata gathers the active window context with a 200ms timeout.
// If the metadata provider takes too long, an empty context is returned so
// capture is never blocked.
func (a *App) collectMetadata() domain.CaptureContext {
	ch := make(chan domain.CaptureContext, 1)
	go func() {
		ch <- domain.CaptureContext{
			WindowTitle: a.meta.GetActiveWindowTitle(),
			AppName:     a.meta.GetActiveAppName(),
			URL:         a.meta.GetActiveURL(),
		}
	}()
	select {
	case ctx := <-ch:
		return ctx
	case <-time.After(200 * time.Millisecond):
		return domain.CaptureContext{}
	}
}

func (a *App) classifyAsync(id int64, content string) {
	ctx := context.Background()

	classification, err := a.ai.ClassifyThought(content)
	if err != nil {
		log.Printf("classifyAsync: %v", err)
		return
	}
	for _, tag := range classification.Tags {
		if err := a.store.AddTag(id, tag.Name, "ai", tag.Confidence); err != nil {
			log.Printf("addTag %q: %v", tag.Name, err)
		}
	}

	// Mishap detection — hide the thought and move it to the "Review Needed" bin.
	if a.ai.IsMishap(content, 0) {
		if err := a.store.HideThought(id); err != nil {
			log.Printf("hideThought %d: %v", id, err)
		} else {
			a.store.AddTag(id, "mishap", "ai", 0.9)
			if !a.testMode && a.ctx != nil {
				runtime.EventsEmit(a.ctx, "mishaps:changed")
			}
		}
	}

	// Embed the thought and store the vector for future semantic search.
	// Times out after 5s so a slow model never delays the classification event.
	embedCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if vec, err := a.ai.Embed(embedCtx, content); err != nil {
		log.Printf("embed thought %d: %v", id, err)
	} else if len(vec) > 0 {
		if err := a.store.StoreEmbedding(id, vec); err != nil {
			log.Printf("storeEmbedding %d: %v", id, err)
		}
	}

	// Notify the frontend that the thought has been enriched.
	if !a.testMode && a.ctx != nil {
		runtime.EventsEmit(a.ctx, "thought:classified", id)
	}
}

// --- Retrieval ---

// GetRecentThoughts returns the N most recently saved thoughts.
func (a *App) GetRecentThoughts(limit int) ([]*domain.Thought, error) {
	if limit <= 0 {
		limit = 5
	}
	return a.store.GetRecentThoughts(limit)
}

// SearchThoughts returns thoughts matching the query string.
// When query is empty, returns the 20 most recent thoughts.
func (a *App) SearchThoughts(query string) ([]*domain.Thought, error) {
	if query == "" {
		return a.store.GetRecentThoughts(20)
	}
	return a.store.SearchThoughts(query, 20)
}

// SemanticSearch returns thoughts ranked by semantic similarity to query.
// When vector embeddings are available (ONNX model loaded), results are ranked
// by cosine similarity. Otherwise falls back to text search.
func (a *App) SemanticSearch(query string) ([]*domain.Thought, error) {
	if query == "" {
		return a.store.GetRecentThoughts(20)
	}

	ctx := context.Background()

	// Try to embed the query and rank by cosine similarity.
	queryVec, err := a.ai.Embed(ctx, query)
	if err != nil || len(queryVec) == 0 {
		// No embedding model — degrade to text search.
		return a.store.SemanticSearch(query, 20)
	}

	// Load candidate thoughts via text pre-filter, then re-rank by vector.
	candidates, err := a.store.GetRecentThoughts(200)
	if err != nil || len(candidates) == 0 {
		return a.store.SemanticSearch(query, 20)
	}

	ids := make([]int64, len(candidates))
	for i, t := range candidates {
		ids[i] = t.ID
	}

	embeddings, err := a.store.GetEmbeddings(ids)
	if err != nil || len(embeddings) == 0 {
		return a.store.SemanticSearch(query, 20)
	}

	type scored struct {
		thought *domain.Thought
		score   float32
	}
	var results []scored
	for _, t := range candidates {
		vec, ok := embeddings[t.ID]
		if !ok {
			continue
		}
		results = append(results, scored{thought: t, score: cosineSimilarity(queryVec, vec)})
	}

	// Sort by descending similarity.
	for i := 1; i < len(results); i++ {
		for j := i; j > 0 && results[j].score > results[j-1].score; j-- {
			results[j], results[j-1] = results[j-1], results[j]
		}
	}

	limit := 20
	if len(results) < limit {
		limit = len(results)
	}
	out := make([]*domain.Thought, limit)
	for i := range out {
		out[i] = results[i].thought
	}
	return out, nil
}

// FindRelated returns a thought that is semantically related to text and was
// captured more than 24 hours ago. Returns nil (no error) when nothing qualifies.
// Called by the frontend after a 1.5 s typing pause (DELTA-3c proactive synthesis).
func (a *App) FindRelated(text string) (*domain.Thought, error) {
	if len(text) < 3 {
		return nil, nil
	}
	candidates, err := a.SemanticSearch(text)
	if err != nil || len(candidates) == 0 {
		return nil, nil
	}
	cutoff := time.Now().Add(-24 * time.Hour)
	for _, t := range candidates {
		if t.CreatedAt.Before(cutoff) {
			return t, nil
		}
	}
	return nil, nil
}

// --- Review Mode actions ---

// UpdateThought edits the visible content of a thought.
// The original text (shadow record) is preserved.
func (a *App) UpdateThought(id int64, content string) (*domain.Thought, error) {
	return a.store.UpdateThought(id, content)
}

// DeleteThought removes a thought permanently.
func (a *App) DeleteThought(id int64) error {
	return a.store.DeleteThought(id)
}

// GetThought returns a single thought by ID.
func (a *App) GetThought(id int64) (*domain.Thought, error) {
	return a.store.GetThought(id)
}

// --- Mishap / "Review Needed" bin ---

// GetHiddenThoughts returns thoughts flagged as mishaps, awaiting user review.
func (a *App) GetHiddenThoughts() ([]*domain.Thought, error) {
	return a.store.GetHiddenThoughts()
}

// UnhideThought moves a thought out of the mishap bin and removes the mishap tag.
func (a *App) UnhideThought(id int64) error {
	return a.store.UnhideThought(id)
}

// --- Window / Mode control ---

// ShowCapture switches to capture mode: small frameless bar, always on top.
// Preserves the current window position so the input bar doesn't jump.
func (a *App) ShowCapture() {
	if a.testMode {
		return
	}
	// Collect metadata before showing the window so we capture the previously
	// active application (not Thawts itself).
	ctx := a.collectMetadata()
	a.captureCtxMu.Lock()
	a.captureCtx = ctx
	a.captureCtxMu.Unlock()

	x, y := runtime.WindowGetPosition(a.ctx)
	runtime.WindowSetSize(a.ctx, captureWidth, captureHeight)
	runtime.WindowSetPosition(a.ctx, x, y)
	runtime.WindowSetAlwaysOnTop(a.ctx, true)
	runtime.WindowShow(a.ctx)
	runtime.EventsEmit(a.ctx, "mode:capture")
}

// ShowReview switches to review mode: larger window, standard chrome.
// Preserves the current window position so the input bar doesn't jump.
func (a *App) ShowReview() {
	if a.testMode {
		return
	}
	x, y := runtime.WindowGetPosition(a.ctx)
	runtime.WindowSetSize(a.ctx, reviewWidth, reviewHeight)
	runtime.WindowSetPosition(a.ctx, x, y)
	runtime.WindowSetAlwaysOnTop(a.ctx, false)
	runtime.WindowShow(a.ctx)
	runtime.EventsEmit(a.ctx, "mode:review")
}

// HideWindow hides the application window.
func (a *App) HideWindow() {
	if a.testMode {
		return
	}
	runtime.WindowHide(a.ctx)
}

// ToggleCapture shows capture mode centered on screen (initial appearance via hotkey).
func (a *App) ToggleCapture() {
	if a.testMode {
		return
	}
	// Collect metadata before showing the window so we capture the previously
	// active application (not Thawts itself).
	ctx := a.collectMetadata()
	a.captureCtxMu.Lock()
	a.captureCtx = ctx
	a.captureCtxMu.Unlock()

	runtime.WindowSetSize(a.ctx, captureWidth, captureHeight)
	runtime.WindowSetAlwaysOnTop(a.ctx, true)
	runtime.WindowCenter(a.ctx)
	runtime.WindowShow(a.ctx)
	runtime.EventsEmit(a.ctx, "mode:capture")
}

// SetCaptureHeight resizes the capture window height as the thought list grows.
func (a *App) SetCaptureHeight(h int) {
	if a.testMode {
		return
	}
	runtime.WindowSetSize(a.ctx, captureWidth, h)
}
