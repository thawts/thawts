package main

import (
	"context"
	"embed"
	"log"
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/menu/keys"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/runtime"

	"thawts-client/internal/storage"

	"golang.design/x/hotkey"
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
	app := NewApp(store)

	// Create Application Menu
	appMenu := menu.NewMenu()

	// App Menu (macOS standard)
	FileMenu := appMenu.AddSubmenu("File")
	FileMenu.AddText("Show Thawts", keys.CmdOrCtrl("o"), func(_ *menu.CallbackData) {
		app.Show()
	})
	FileMenu.AddSeparator()
	FileMenu.AddText("Quit", keys.CmdOrCtrl("q"), func(_ *menu.CallbackData) {
		runtime.Quit(app.ctx)
	})

	// Edit Menu (Standard)
	EditMenu := appMenu.AddSubmenu("Edit")
	EditMenu.AddText("Undo", keys.CmdOrCtrl("z"), func(_ *menu.CallbackData) { runtime.WindowExecJS(app.ctx, "document.execCommand('undo')") })
	EditMenu.AddText("Redo", keys.CmdOrCtrl("y"), func(_ *menu.CallbackData) { runtime.WindowExecJS(app.ctx, "document.execCommand('redo')") })
	EditMenu.AddSeparator()
	EditMenu.AddText("Cut", keys.CmdOrCtrl("x"), func(_ *menu.CallbackData) { runtime.WindowExecJS(app.ctx, "document.execCommand('cut')") })
	EditMenu.AddText("Copy", keys.CmdOrCtrl("c"), func(_ *menu.CallbackData) { runtime.WindowExecJS(app.ctx, "document.execCommand('copy')") })
	EditMenu.AddText("Paste", keys.CmdOrCtrl("v"), func(_ *menu.CallbackData) { runtime.WindowExecJS(app.ctx, "document.execCommand('paste')") })
	EditMenu.AddText("Select All", keys.CmdOrCtrl("a"), func(_ *menu.CallbackData) { runtime.WindowExecJS(app.ctx, "document.execCommand('selectAll')") })

	// Data Menu
	DataMenu := appMenu.AddSubmenu("Data")
	DataMenu.AddText("Export Data...", nil, func(_ *menu.CallbackData) {
		app.ExportThoughts()
	})
	DataMenu.AddText("Import Data...", nil, func(_ *menu.CallbackData) {
		app.ImportThoughts()
	})

	// Create application with options
	err = wails.Run(&options.App{
		Title:       "thawts-client",
		Width:       800,
		Height:      60,
		Frameless:   true,
		AlwaysOnTop: true,
		Menu:        appMenu,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 0, G: 0, B: 0, A: 0},
		OnStartup: func(ctx context.Context) {
			app.startup(ctx)
			runtime.WindowHide(ctx)

			// Init Tray
			RegisterApp(app)
			InitTray()

			// Register hotkey: Ctrl+Shift+Space
			// Mods: Ctrl (Command/Meta), Shift
			// Key: Space
			// Register hotkey: Ctrl+Shift+Space
			// Mods: Ctrl (Command/Meta), Shift
			// Key: Space
			go func() {
				// Ctrl+Shift+Space
				hk := hotkey.New([]hotkey.Modifier{hotkey.ModCtrl, hotkey.ModShift}, hotkey.KeySpace)
				if err := hk.Register(); err != nil {
					log.Println("failed to register hotkey:", err)
					return
				}
				for range hk.Keydown() {
					app.Toggle()
				}
			}()
		},
		Bind: []interface{}{
			app,
		},
		Mac: &mac.Options{
			TitleBar:             mac.TitleBarHiddenInset(),
			Appearance:           mac.NSAppearanceNameDarkAqua,
			WebviewIsTransparent: true,
			WindowIsTranslucent:  true,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
