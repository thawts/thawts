// Package app contains the Wails application adapter.
//
// App embeds *service.Service so all business methods are reachable from the
// frontend via the generated JS bindings. Window control methods live here
// because they are Wails-specific and have no place in the framework-agnostic
// service layer.
package app

import (
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/thawts/thawts/internal/service"
)

const (
	windowWidth   = 1200
	captureHeight = 60
	reviewHeight  = 750
)

// App is the Wails application adapter. Embed service.Service is promoted so
// all its exported methods are callable from the frontend via generated JS bindings.
type App struct {
	*service.Service
	wailsApp    *application.App
	window      *application.WebviewWindow
	isCapturing bool
	shownAt     time.Time
}

// NewApp constructs the Wails adapter with its dependencies.
func NewApp(wailsApp *application.App, window *application.WebviewWindow, svc *service.Service) *App {
	return &App{
		Service:  svc,
		wailsApp: wailsApp,
		window:   window,
	}
}

// IsCapturing reports whether the window is currently in capture mode.
func (a *App) IsCapturing() bool {
	return a.isCapturing
}

// ShouldHideOnFocusLoss reports whether the focus-loss hook should hide the
// window. Returns false for a short grace period after the window is shown so
// that the tray-click or hotkey interaction does not immediately steal focus
// back before the window has a chance to become active.
func (a *App) ShouldHideOnFocusLoss() bool {
	return a.isCapturing && time.Since(a.shownAt) > 300*time.Millisecond
}

// Quit shuts down the application.
func (a *App) Quit() {
	a.wailsApp.Quit()
}

// ShowCapture switches to capture mode: thin bar, always on top.
func (a *App) ShowCapture() {
	a.PrepareCapture()
	x, y := a.window.Position()
	a.window.SetSize(windowWidth, captureHeight)
	a.window.SetPosition(x, y)
	a.window.SetAlwaysOnTop(true)
	a.shownAt = time.Now()
	a.isCapturing = true
	a.window.Show()
	a.window.Focus()
	a.wailsApp.Event.Emit("mode:capture")
}

// ShowReview switches to review mode, expanding the window downward.
func (a *App) ShowReview() {
	x, y := a.window.Position()
	a.window.SetSize(windowWidth, reviewHeight)
	a.window.SetPosition(x, y)
	a.window.SetAlwaysOnTop(false)
	a.window.Show()
	a.window.Focus()
	a.wailsApp.Event.Emit("mode:review")
	a.isCapturing = false
}

// HideWindow hides the application window.
func (a *App) HideWindow() {
	a.window.Hide()
	a.isCapturing = false
}

// ToggleCapture toggles capture mode via the global hotkey.
// If the window is already visible in capture mode it is hidden; otherwise it
// is centered on screen and shown. Centers using review dimensions first so
// that expanding to review later only changes the height.
func (a *App) ToggleCapture() {
	if a.isCapturing {
		a.HideWindow()
		return
	}
	a.PrepareCapture()
	a.window.SetSize(windowWidth, reviewHeight)
	a.window.Center()
	x, y := a.window.Position()
	a.window.SetSize(windowWidth, captureHeight)
	a.window.SetPosition(x, y)
	a.window.SetAlwaysOnTop(true)
	a.shownAt = time.Now()
	a.isCapturing = true
	a.window.Show()
	a.window.Focus()
	a.wailsApp.Event.Emit("mode:capture")
}

// SetCaptureHeight resizes the capture window height as the thought list grows.
func (a *App) SetCaptureHeight(h int) {
	a.window.SetSize(windowWidth, h)
}
