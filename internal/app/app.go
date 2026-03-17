// Package app contains the Wails-bound application struct.
package app

import (
	"context"
	"log"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"thawts-client/internal/ai"
	"thawts-client/internal/domain"
	"thawts-client/internal/metadata"
	"thawts-client/internal/storage"
)

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
	ctx := domain.CaptureContext{
		WindowTitle: a.meta.GetActiveWindowTitle(),
		AppName:     a.meta.GetActiveAppName(),
		URL:         a.meta.GetActiveURL(),
	}

	thought, err := a.store.SaveThought(content, ctx)
	if err != nil {
		return nil, err
	}

	// Classify in the background so the UI is never blocked.
	go a.classifyAsync(thought.ID, content)

	return thought, nil
}

func (a *App) classifyAsync(id int64, content string) {
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

// --- Window / Mode control ---

// ShowCapture switches to capture mode: small frameless bar, always on top.
// Preserves the current window position so the input bar doesn't jump.
func (a *App) ShowCapture() {
	if a.testMode {
		return
	}
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
