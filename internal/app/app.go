package app

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"

	"sync"
	"thawts-client/internal/storage"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct
type App struct {
	ctx     context.Context
	storage *storage.Service

	// Interaction state
	interactLock  sync.Mutex
	isInteracting bool
	isVisible     bool

	// Test mode (to skip runtime calls)
	testMode bool
}

// SetTestMode enables or disables test mode
func (a *App) SetTestMode(enabled bool) {
	a.testMode = enabled
}

// NewApp creates a new App application struct
func NewApp(storageService *storage.Service) *App {
	return &App{
		storage: storageService,
	}
}

// Startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
}

func (a *App) Context() context.Context {
	return a.ctx
}

// Save saves the thought to local storage
func (a *App) Save(text string) error {
	if text == "" {
		return nil
	}
	err := a.storage.SaveThought(text)
	if err != nil {
		return err
	}
	// a.Hide() - User wants app to stay open for multiple entries
	return nil
}

// Greet returns a greeting for the given name
func (a *App) Greet(name string) string {
	return fmt.Sprintf("Hello %s, It's show time!", name)
}

func (a *App) Hide() {
	if a.testMode {
		a.isVisible = false
		return
	}
	a.interactLock.Lock()
	if a.isInteracting {
		a.interactLock.Unlock()
		return
	}
	a.interactLock.Unlock()

	a.isVisible = false
	runtime.WindowHide(a.ctx)
}

func (a *App) Show() {
	if a.testMode {
		a.isVisible = true
		return
	}
	a.isVisible = true
	// Force unminimise/restore logic for Windows to ensure it grabs attention
	runtime.WindowUnminimise(a.ctx)
	runtime.WindowShow(a.ctx)
	runtime.WindowSetAlwaysOnTop(a.ctx, true)
	// Emit event for frontend to focus input
	runtime.EventsEmit(a.ctx, "window-shown")
}

func (a *App) Toggle() {
	if a.isVisible {
		a.Hide()
	} else {
		a.Show()
	}
}

func (a *App) Quit() {
	if a.testMode {
		return
	}
	runtime.Quit(a.ctx)
}

// Search returns thoughts matching the query
func (a *App) Search(query string) []storage.Thought {
	if query == "" {
		return nil
	}
	thoughts, err := a.storage.SearchThoughts(query)
	if err != nil {
		// Log error if needed, but for now just return nil/empty
		return nil
	}
	return thoughts
}

// SetWindowHeight sets the window height
func (a *App) SetWindowHeight(height int) {
	if a.testMode {
		return
	}
	width, _ := runtime.WindowGetSize(a.ctx)
	runtime.WindowSetSize(a.ctx, width, height)
}

// ExportThoughts handles the export workflow
func (a *App) ExportThoughts() {
	// Interaction Start
	a.interactLock.Lock()
	a.isInteracting = true
	a.interactLock.Unlock()

	defer func() {
		a.interactLock.Lock()
		a.isInteracting = false
		a.interactLock.Unlock()
	}()

	// Ensure Visible
	if !a.testMode {
		runtime.WindowShow(a.ctx)
	}

	// Select File
	if a.testMode {
		// Mock file selection in test mode if needed, or just return/error
		// For now, let's just return to avoid runtime panic
		return
	}
	file, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "Export Data",
		DefaultFilename: "thawts_export.json",
		Filters: []runtime.FileFilter{
			{DisplayName: "JSON Files (*.json)", Pattern: "*.json"},
			{DisplayName: "CSV Files (*.csv)", Pattern: "*.csv"},
		},
	})
	if err != nil || file == "" {
		return
	}

	f, err := os.Create(file)
	if err != nil {
		runtime.MessageDialog(a.ctx, runtime.MessageDialogOptions{
			Type:    runtime.ErrorDialog,
			Title:   "Export Failed",
			Message: err.Error(),
		})
		return
	}
	defer f.Close()

	ext := filepath.Ext(file)
	if ext == ".csv" {
		w := csv.NewWriter(f)
		err = a.storage.ExportCSV(w)
	} else {
		// Default to JSON
		err = a.storage.ExportJSONToWriter(f)
	}

	if err != nil {
		runtime.MessageDialog(a.ctx, runtime.MessageDialogOptions{
			Type:    runtime.ErrorDialog,
			Title:   "Export Failed",
			Message: err.Error(),
		})
	} else {
		runtime.MessageDialog(a.ctx, runtime.MessageDialogOptions{
			Type:    runtime.InfoDialog,
			Title:   "Export Successful",
			Message: "Your thoughts have been exported.",
		})
	}
}

// ImportThoughts handles the import workflow
func (a *App) ImportThoughts() {
	// Interaction Start
	a.interactLock.Lock()
	a.isInteracting = true
	a.interactLock.Unlock()

	defer func() {
		a.interactLock.Lock()
		a.isInteracting = false
		a.interactLock.Unlock()
	}()

	// Ensure Visible
	if !a.testMode {
		runtime.WindowShow(a.ctx)
	}

	// Select File
	if a.testMode {
		return
	}
	file, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Import Data",
		Filters: []runtime.FileFilter{
			{DisplayName: "Data Files (*.json;*.csv)", Pattern: "*.json;*.csv"},
		},
	})
	if err != nil || file == "" {
		return
	}

	// Ask to Remove Existing
	selection, err := runtime.MessageDialog(a.ctx, runtime.MessageDialogOptions{
		Type:          runtime.QuestionDialog,
		Title:         "Import Options",
		Message:       "Do you want to clear existing data before importing?",
		Buttons:       []string{"Yes, Clear Data", "No, Append"},
		DefaultButton: "No, Append",
		CancelButton:  "Cancel",
	})

	removeExisting := false
	if selection == "Yes, Clear Data" {
		removeExisting = true
	} else if selection != "No, Append" {
		// Cancelled?
		return
	}

	f, err := os.Open(file)
	if err != nil {
		runtime.MessageDialog(a.ctx, runtime.MessageDialogOptions{
			Type:    runtime.ErrorDialog,
			Title:   "Import Failed",
			Message: err.Error(),
		})
		return
	}
	defer f.Close()

	ext := filepath.Ext(file)
	if ext == ".csv" {
		r := csv.NewReader(f)
		err = a.storage.ImportCSV(r, removeExisting)
	} else {
		err = a.storage.ImportJSON(f, removeExisting)
	}

	if err != nil {
		runtime.MessageDialog(a.ctx, runtime.MessageDialogOptions{
			Type:    runtime.ErrorDialog,
			Title:   "Import Failed",
			Message: err.Error(),
		})
	} else {
		runtime.MessageDialog(a.ctx, runtime.MessageDialogOptions{
			Type:    runtime.InfoDialog,
			Title:   "Import Successful",
			Message: "Your thoughts have been imported.",
		})
	}
}
