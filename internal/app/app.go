// Package app contains the Wails application adapter.
//
// App embeds *service.Service so all business methods are reachable from the
// frontend via the generated JS bindings. Window control methods live here
// because they are Wails-specific and have no place in the framework-agnostic
// service layer.
package app

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sync/atomic"
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
	dialogOpen  atomic.Int32 // >0 while a native file dialog is open
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

// IsDialogOpen reports whether a native file dialog is currently open.
// The focus-loss hide hook must not hide the window while a dialog is open.
func (a *App) IsDialogOpen() bool {
	return a.dialogOpen.Load() > 0
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
	log.Printf("ToggleCapture called (isCapturing=%v)", a.isCapturing)
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

// promptSaveFile shows a save-file dialog, temporarily disabling always-on-top
// so the dialog is not obscured by the Thawts window.
func (a *App) promptSaveFile(d *application.SaveFileDialogStruct) (string, error) {
	a.window.SetAlwaysOnTop(false)
	a.dialogOpen.Add(1)
	path, err := d.PromptForSingleSelection()
	a.dialogOpen.Add(-1)
	if a.isCapturing {
		a.window.SetAlwaysOnTop(true)
	}
	return path, err
}

// promptOpenFile shows an open-file dialog, temporarily disabling always-on-top.
func (a *App) promptOpenFile(d *application.OpenFileDialogStruct) (string, error) {
	a.window.SetAlwaysOnTop(false)
	a.dialogOpen.Add(1)
	path, err := d.PromptForSingleSelection()
	a.dialogOpen.Add(-1)
	if a.isCapturing {
		a.window.SetAlwaysOnTop(true)
	}
	return path, err
}

// ExportJSON shows a save-file dialog and exports all thoughts to JSON.
// Returns a human-readable summary or an error message.
func (a *App) ExportJSON() (string, error) {
	path, err := a.promptSaveFile(a.wailsApp.Dialog.SaveFile().
		AddFilter("JSON files", "*.json").
		SetFilename("thawts-export.json"))
	if err != nil || path == "" {
		return "", nil // cancelled
	}
	n, err := a.Service.ExportToJSON(path)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Exported %d thoughts → %s", n, filepath.Base(path)), nil
}

// ExportCSV shows a save-file dialog and exports all thoughts to CSV.
func (a *App) ExportCSV() (string, error) {
	path, err := a.promptSaveFile(a.wailsApp.Dialog.SaveFile().
		AddFilter("CSV files", "*.csv").
		SetFilename("thawts-export.csv"))
	if err != nil || path == "" {
		return "", nil
	}
	n, err := a.Service.ExportToCSV(path)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Exported %d thoughts → %s", n, filepath.Base(path)), nil
}

// ImportJSON shows an open-file dialog and imports thoughts from a JSON file.
// When restore is true all existing data is deleted before importing.
func (a *App) ImportJSON(restore bool) (string, error) {
	path, err := a.promptOpenFile(a.wailsApp.Dialog.OpenFile().
		AddFilter("JSON files", "*.json"))
	if err != nil || path == "" {
		return "", nil
	}
	n, err := a.Service.ImportFromJSON(path, restore)
	if err != nil {
		return "", err
	}
	mode := "added"
	if restore {
		mode = "restored"
	}
	a.wailsApp.Event.Emit("thoughts:imported")
	return fmt.Sprintf("Successfully %s %d thoughts", mode, n), nil
}

// ImportCSV shows an open-file dialog and imports thoughts from a CSV file.
// When restore is true all existing data is deleted before importing.
func (a *App) ImportCSV(restore bool) (string, error) {
	path, err := a.promptOpenFile(a.wailsApp.Dialog.OpenFile().
		AddFilter("CSV files", "*.csv"))
	if err != nil || path == "" {
		return "", nil
	}
	n, err := a.Service.ImportFromCSV(path, restore)
	if err != nil {
		return "", err
	}
	mode := "added"
	if restore {
		mode = "restored"
	}
	a.wailsApp.Event.Emit("thoughts:imported")
	return fmt.Sprintf("Successfully %s %d thoughts", mode, n), nil
}

// RestartApp launches a fresh copy of the binary and quits the current process.
func (a *App) RestartApp() {
	exe, err := os.Executable()
	if err != nil {
		log.Printf("RestartApp: %v", err)
		return
	}
	cmd := exec.Command(exe, os.Args[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()
	if err := cmd.Start(); err != nil {
		log.Printf("RestartApp start: %v", err)
		return
	}
	a.wailsApp.Quit()
}
