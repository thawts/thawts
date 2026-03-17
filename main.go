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
	"golang.design/x/hotkey"

	"thawts-client/internal/ai"
	"thawts-client/internal/app"
	"thawts-client/internal/metadata"
	"thawts-client/internal/storage"
	"thawts-client/internal/tray"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/appicon.png
var appIcon []byte

func main() {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
	}

	store, err := storage.NewSQLiteStorage(filepath.Join(homeDir, ".thawts", "thawts.db"))
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()

	application := app.NewApp(
		store,
		ai.NewLLMProvider(filepath.Join(homeDir, ".thawts", "models", "classifier.gguf")),
		metadata.New(),
	)

	appMenu := buildMenu(application)

	err = wails.Run(&options.App{
		Title:       "Thawts",
		Width:       800,
		Height:      60,
		Frameless:   true,
		AlwaysOnTop:      true,
		HideWindowOnClose: true,
		Menu:             appMenu,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 0, G: 0, B: 0, A: 255},
		OnStartup: func(ctx context.Context) {
			application.Startup(ctx)
			runtime.WindowHide(ctx)

			tray.Init(application, appIcon)

			// Ctrl+Shift+Space → toggle capture mode
			go registerHotkey(
				[]hotkey.Modifier{hotkey.ModShift, hotkey.ModCtrl},
				hotkey.KeySpace,
				application.ToggleCapture,
			)

			// Cmd+Option+R → open review mode
			go registerHotkey(
				[]hotkey.Modifier{hotkey.ModCmd, hotkey.ModOption},
				hotkey.KeyR,
				application.ShowReview,
			)
		},
		Bind: []interface{}{application},
		Mac: &mac.Options{
			TitleBar:             mac.TitleBarHidden(),
			Appearance:           mac.NSAppearanceNameDarkAqua,
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
		},
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId: "e1db439e-43e1-4119-880e-37e47522e90d",
			OnSecondInstanceLaunch: func(_ options.SecondInstanceData) {
				application.ShowCapture()
			},
		},
	})

	if err != nil {
		log.Fatal(err)
	}
}

func buildMenu(application *app.App) *menu.Menu {
	m := menu.NewMenu()

	file := m.AddSubmenu("File")
	file.AddText("Capture Thought", keys.CmdOrCtrl("o"), func(_ *menu.CallbackData) {
		application.ShowCapture()
	})
	file.AddText("Review Garden", keys.Combo("r", keys.CmdOrCtrlKey, keys.OptionOrAltKey), func(_ *menu.CallbackData) {
		application.ShowReview()
	})
	file.AddSeparator()
	file.AddText("Quit", keys.CmdOrCtrl("q"), func(_ *menu.CallbackData) {
		application.Quit()
	})

	edit := m.AddSubmenu("Edit")
	edit.AddText("Cut", keys.CmdOrCtrl("x"), func(_ *menu.CallbackData) {
		runtime.WindowExecJS(application.Context(), "document.execCommand('cut')")
	})
	edit.AddText("Copy", keys.CmdOrCtrl("c"), func(_ *menu.CallbackData) {
		runtime.WindowExecJS(application.Context(), "document.execCommand('copy')")
	})
	edit.AddText("Paste", keys.CmdOrCtrl("v"), func(_ *menu.CallbackData) {
		runtime.WindowExecJS(application.Context(), "document.execCommand('paste')")
	})
	edit.AddText("Select All", keys.CmdOrCtrl("a"), func(_ *menu.CallbackData) {
		runtime.WindowExecJS(application.Context(), "document.execCommand('selectAll')")
	})

	return m
}

func registerHotkey(mods []hotkey.Modifier, key hotkey.Key, fn func()) {
	hk := hotkey.New(mods, key)
	if err := hk.Register(); err != nil {
		log.Printf("hotkey register failed: %v", err)
		return
	}
	for range hk.Keydown() {
		fn()
	}
}
