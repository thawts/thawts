package main

import (
	"embed"
	"log"
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/menu/keys"
	"github.com/wailsapp/wails/v2/pkg/runtime"

	"thawts-client/internal/app"
	"thawts-client/internal/storage"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// Initialize Storage
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
	}
	dbPath := filepath.Join(homeDir, ".thawts", "thawts.db")

	store, err := storage.NewService(dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()

	// Create an instance of the app structure
	application := app.NewApp(store)

	// Create Application Menu
	appMenu := menu.NewMenu()

	// App Menu (macOS standard)
	FileMenu := appMenu.AddSubmenu("File")
	FileMenu.AddText("Show Thawts", keys.CmdOrCtrl("o"), func(_ *menu.CallbackData) {
		application.Show()
	})
	FileMenu.AddSeparator()
	FileMenu.AddText("Quit", keys.CmdOrCtrl("q"), func(_ *menu.CallbackData) {
		runtime.Quit(application.Context()) // Wait, we need to access context or just use Quit method?
		// app.Quit() wraps runtime.Quit(a.ctx)
		application.Quit()
	})

	// Edit Menu (Standard)
	EditMenu := appMenu.AddSubmenu("Edit")
	EditMenu.AddText("Undo", keys.CmdOrCtrl("z"), func(_ *menu.CallbackData) {
		runtime.WindowExecJS(application.Context(), "document.execCommand('undo')")
	})
	EditMenu.AddText("Redo", keys.CmdOrCtrl("y"), func(_ *menu.CallbackData) {
		runtime.WindowExecJS(application.Context(), "document.execCommand('redo')")
	})
	EditMenu.AddSeparator()
	EditMenu.AddText("Cut", keys.CmdOrCtrl("x"), func(_ *menu.CallbackData) { runtime.WindowExecJS(application.Context(), "document.execCommand('cut')") })
	EditMenu.AddText("Copy", keys.CmdOrCtrl("c"), func(_ *menu.CallbackData) {
		runtime.WindowExecJS(application.Context(), "document.execCommand('copy')")
	})
	EditMenu.AddText("Paste", keys.CmdOrCtrl("v"), func(_ *menu.CallbackData) {
		runtime.WindowExecJS(application.Context(), "document.execCommand('paste')")
	})
	EditMenu.AddText("Select All", keys.CmdOrCtrl("a"), func(_ *menu.CallbackData) {
		runtime.WindowExecJS(application.Context(), "document.execCommand('selectAll')")
	})

	// Data Menu
	DataMenu := appMenu.AddSubmenu("Data")
	DataMenu.AddText("Export Data...", nil, func(_ *menu.CallbackData) {
		application.ExportThoughts()
	})
	DataMenu.AddText("Import Data...", nil, func(_ *menu.CallbackData) {
		application.ImportThoughts()
	})
}
