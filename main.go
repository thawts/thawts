package main

import (
	"context"
	"embed"
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/runtime"

	"golang.design/x/hotkey"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// Create an instance of the app structure
	app := NewApp()

	// Create application with options
	err := wails.Run(&options.App{
		Title:       "thawts-client",
		Width:       800,
		Height:      60,
		Frameless:   true,
		AlwaysOnTop: true,
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
