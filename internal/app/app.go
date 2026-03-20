// Package app contains the Wails application adapter.
//
// App embeds *service.Service so all business methods are reachable from the
// frontend via the generated JS bindings. Window control methods live here
// because they are Wails-specific and have no place in the framework-agnostic
// service layer.
package app

import (
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
// Used by the focus-loss hook to decide whether to auto-hide.
func (a *App) IsCapturing() bool {
	return a.isCapturing
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
	a.window.Show()
	a.window.Focus()
	a.wailsApp.Event.Emit("mode:capture")
	a.isCapturing = true
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

// ToggleCapture shows capture mode via the global hotkey, centering the window.
// Centers using review dimensions first so that expanding to review later only
// changes the height — the top-left position stays identical.
func (a *App) ToggleCapture() {
	a.PrepareCapture()
	a.window.SetSize(windowWidth, reviewHeight)
	a.window.Center()
	x, y := a.window.Position()
	a.window.SetSize(windowWidth, captureHeight)
	a.window.SetPosition(x, y)
	a.window.SetAlwaysOnTop(true)
	a.window.Show()
	a.window.Focus()
	a.wailsApp.Event.Emit("mode:capture")
	a.isCapturing = true
}

// SetCaptureHeight resizes the capture window height as the thought list grows.
func (a *App) SetCaptureHeight(h int) {
	a.window.SetSize(windowWidth, h)
}
