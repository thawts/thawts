// Package app contains the Wails-bound application struct.
package app

import (
	"context"
	"fmt"
	gort "runtime"
	"log"
	"math"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

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
	windowWidth   = 1200
	captureHeight = 60
	reviewHeight  = 750
)

// App is the Wails application struct. All exported methods are callable from
// the frontend via the generated JS bindings.
type App struct {
	wailsApp *application.App
	window   *application.WebviewWindow
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
// Pass nil for wailsApp and window when constructing for unit tests (use SetTestMode(true)).
func NewApp(wailsApp *application.App, window *application.WebviewWindow, store storage.Storage, aiProvider ai.Provider, metaProvider metadata.Provider) *App {
	return &App{
		wailsApp: wailsApp,
		window:   window,
		store:    store,
		ai:       aiProvider,
		meta:     metaProvider,
	}
}

// Quit shuts down the application.
func (a *App) Quit() {
	if !a.testMode {
		a.wailsApp.Quit()
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
			if !a.testMode {
				a.wailsApp.Event.Emit("mishaps:changed")
			}
		}
	} else {
		// Intent detection — only for non-mishap thoughts.
		intents, err := a.ai.DetectIntents(content)
		if err == nil && len(intents) > 0 {
			now := time.Now().UTC()
			saved := 0
			for i, intent := range intents {
				domainIntent := domain.Intent{
					ID:        fmt.Sprintf("%d-%s-%d", id, intent.Type, i),
					ThoughtID: id,
					Type:      intent.Type,
					Title:     intent.Description,
					Status:    "pending",
					CreatedAt: now,
				}
				if err := a.store.SaveIntent(domainIntent); err != nil {
					log.Printf("saveIntent: %v", err)
				} else {
					saved++
				}
			}
			if saved > 0 && !a.testMode {
				a.wailsApp.Event.Emit("intents:changed")
			}
		}
	}

	// Analyze sentiment and store.
	sentimentCtx, cancelSentiment := context.WithTimeout(ctx, 2*time.Second)
	defer cancelSentiment()
	if score, err := a.ai.AnalyzeSentiment(sentimentCtx, content); err != nil {
		log.Printf("analyzeSentiment %d: %v", id, err)
	} else {
		if err := a.store.StoreSentiment(id, score); err != nil {
			log.Printf("storeSentiment %d: %v", id, err)
		} else {
			a.checkWellbeingTrend()
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
	if !a.testMode {
		a.wailsApp.Event.Emit("thought:classified", id)
	}
}

// checkWellbeingTrend computes the 7-day rolling sentiment average and emits
// a wellbeing:alert event when it falls below the burnout threshold (−0.4).
func (a *App) checkWellbeingTrend() {
	avg, err := a.GetSentimentTrend(7)
	if err != nil || avg == 0 {
		return
	}
	if avg < -0.4 {
		if !a.testMode {
			a.wailsApp.Event.Emit("wellbeing:alert", avg)
		}
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

	limit := min(20, len(results))
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

// MergeThoughts combines multiple thoughts into one, soft-deleting the originals.
func (a *App) MergeThoughts(ids []int64) (*domain.Thought, error) {
	merged, err := a.store.MergeThoughts(ids)
	if err != nil {
		return nil, err
	}
	if !a.testMode {
		a.wailsApp.Event.Emit("thoughts:merged")
	}
	return merged, nil
}

// CleanText returns a typo-corrected version of the thought's content for
// display purposes only. The original content is never modified.
func (a *App) CleanText(id int64) (string, error) {
	thought, err := a.store.GetThought(id)
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return a.ai.CleanText(ctx, thought.Content)
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

// --- Intent Actions ---

// GetPendingIntents returns all intents awaiting user confirmation.
func (a *App) GetPendingIntents() ([]domain.Intent, error) {
	return a.store.GetPendingIntents()
}

// ConfirmIntent marks an intent as confirmed and attempts to create a native
// calendar or reminder entry using AppleScript (macOS only, best-effort).
func (a *App) ConfirmIntent(intentID string) error {
	if err := a.store.ConfirmIntent(intentID); err != nil {
		return err
	}
	// Best-effort: create native calendar/reminder entry.
	intent, err := a.store.GetIntent(intentID)
	if err == nil {
		go a.createNativeEvent(intent)
	}
	if !a.testMode {
		a.wailsApp.Event.Emit("intents:changed")
	}
	return nil
}

// DismissIntent marks an intent as dismissed.
func (a *App) DismissIntent(intentID string) error {
	if err := a.store.DismissIntent(intentID); err != nil {
		return err
	}
	if !a.testMode {
		a.wailsApp.Event.Emit("intents:changed")
	}
	return nil
}

// createNativeEvent creates a native calendar or reminder entry via AppleScript
// on macOS. This is a best-effort operation; errors are logged but not surfaced.
func (a *App) createNativeEvent(intent *domain.Intent) {
	if gort.GOOS != "darwin" {
		return
	}
	// Sanitise title for AppleScript: escape double quotes.
	title := strings.ReplaceAll(intent.Title, `"`, `'`)

	var script string
	switch intent.Type {
	case "calendar":
		script = fmt.Sprintf(`tell application "Calendar"
			tell calendar "Calendar"
				make new event at end of events with properties {summary:"%s"}
			end tell
		end tell`, title)
	case "task", "reminder":
		script = fmt.Sprintf(`tell application "Reminders"
			tell list "Reminders"
				make new reminder with properties {name:"%s"}
			end tell
		end tell`, title)
	default:
		return
	}

	cmd := exec.Command("osascript", "-e", script)
	if err := cmd.Run(); err != nil {
		log.Printf("createNativeEvent %q: %v", intent.Title, err)
	}
}

// --- Wellbeing ---

// GetSentimentTrend returns the rolling average sentiment score over the last
// `days` days. Returns 0 when no signals are recorded.
func (a *App) GetSentimentTrend(days int) (float32, error) {
	if days <= 0 {
		days = 7
	}
	signals, err := a.store.GetSentimentTrend(days)
	if err != nil || len(signals) == 0 {
		return 0, err
	}
	var sum float32
	for _, s := range signals {
		sum += s.Score
	}
	return sum / float32(len(signals)), nil
}

// --- Window / Mode control ---

// ShowCapture switches to capture mode: thin bar, always on top.
// Width stays the same as review mode so the input field never moves.
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

	x, y := a.window.Position()
	a.window.SetSize(windowWidth, captureHeight)
	a.window.SetPosition(x, y) // re-pin top after SetSize shifts the anchor
	a.window.SetAlwaysOnTop(true)
	a.window.Show()
	a.window.Focus()
	a.wailsApp.Event.Emit("mode:capture")
}

// ShowReview switches to review mode, expanding the window downward.
// X position is unchanged so the input bar stays at the same screen location.
func (a *App) ShowReview() {
	if a.testMode {
		return
	}
	x, y := a.window.Position()
	a.window.SetSize(windowWidth, reviewHeight)
	a.window.SetPosition(x, y) // re-pin top after SetSize shifts the anchor
	a.window.SetAlwaysOnTop(false)
	a.window.Show()
	a.window.Focus()
	a.wailsApp.Event.Emit("mode:review")
}

// HideWindow hides the application window.
func (a *App) HideWindow() {
	if a.testMode {
		return
	}
	a.window.Hide()
}

// ToggleCapture shows capture mode via the global hotkey.
// The window is centered using review dimensions so that expanding to review
// later only changes the height — the top-left position stays identical.
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

	// Center using review dimensions to get the top-left position that will
	// make the review window perfectly centered. Then shrink to capture height
	// while keeping that same top position, so the two modes share a position.
	a.window.SetSize(windowWidth, reviewHeight)
	a.window.Center()
	x, y := a.window.Position() // top-origin coordinates
	a.window.SetSize(windowWidth, captureHeight)
	a.window.SetPosition(x, y) // pin the top back — SetSize anchors bottom-left, not top
	a.window.SetAlwaysOnTop(true)
	a.window.Show()
	a.window.Focus()
	a.wailsApp.Event.Emit("mode:capture")
}

// SetCaptureHeight resizes the capture window height as the thought list grows.
func (a *App) SetCaptureHeight(h int) {
	if a.testMode {
		return
	}
	a.window.SetSize(windowWidth, h)
}
