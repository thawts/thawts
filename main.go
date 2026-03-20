package main

import (
	"embed"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	"golang.design/x/hotkey"

	thawtsapp "thawts-client/internal/app"
	"thawts-client/internal/ai"
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

	assetsFS, err := fs.Sub(assets, "frontend/dist")
	if err != nil {
		log.Fatal(err)
	}

	// thawtsApp is declared early so the SingleInstance closure can capture it.
	// It is assigned before app.Run(), so it will be set by the time the callback fires.
	var app *thawtsapp.App

	wailsApp := application.New(application.Options{
		Name:        "Thawts",
		Description: "Thought capture and review",
		Assets: application.AssetOptions{
			Handler: http.FileServerFS(assetsFS),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: false,
		},
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID: "e1db439e-43e1-4119-880e-37e47522e90d",
			OnSecondInstanceLaunch: func(_ application.SecondInstanceData) {
				if app != nil {
					app.ShowCapture()
				}
			},
		},
	})

	win := wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "Thawts",
		Width:            1200,
		Height:           60,
		Frameless:        true,
		AlwaysOnTop:      true,
		Hidden:           true,
		BackgroundColour: application.RGBA{Red: 0, Green: 0, Blue: 0, Alpha: 255},
		UseApplicationMenu: true,
		Mac: application.MacWindow{
			TitleBar:   application.MacTitleBarHidden,
			Appearance: application.NSAppearanceNameDarkAqua,
		},
	})

	app = thawtsapp.NewApp(
		wailsApp,
		win,
		store,
		ai.NewLLMProvider(filepath.Join(homeDir, ".thawts", "models", "classifier.gguf")),
		metadata.New(),
	)

	wailsApp.RegisterService(application.NewService(app))

	// Hide instead of close — keeps the app alive in the tray.
	win.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		e.Cancel()
		win.Hide()
	})

	buildMenu(wailsApp, win, app)
	tray.Init(wailsApp, appIcon, app)

	// Ctrl+Shift+Space → toggle capture mode
	go registerHotkey(
		[]hotkey.Modifier{hotkey.ModShift, hotkey.ModCtrl},
		hotkey.KeySpace,
		app.ToggleCapture,
	)

	// Cmd+Option+R → open review mode
	go registerHotkey(
		[]hotkey.Modifier{hotkey.ModCmd, hotkey.ModOption},
		hotkey.KeyR,
		app.ShowReview,
	)

	if err := wailsApp.Run(); err != nil {
		log.Fatal(err)
	}
}

func buildMenu(wailsApp *application.App, win *application.WebviewWindow, app *thawtsapp.App) {
	m := application.NewMenu()

	file := m.AddSubmenu("File")
	file.Add("Capture Thought").OnClick(func(*application.Context) {
		app.ShowCapture()
	})
	file.Add("Review Garden").OnClick(func(*application.Context) {
		app.ShowReview()
	})
	file.AddSeparator()
	file.Add("Quit").OnClick(func(*application.Context) {
		app.Quit()
	})

	edit := m.AddSubmenu("Edit")
	edit.Add("Cut").OnClick(func(*application.Context) {
		win.ExecJS("document.execCommand('cut')")
	})
	edit.Add("Copy").OnClick(func(*application.Context) {
		win.ExecJS("document.execCommand('copy')")
	})
	edit.Add("Paste").OnClick(func(*application.Context) {
		win.ExecJS("document.execCommand('paste')")
	})
	edit.Add("Select All").OnClick(func(*application.Context) {
		win.ExecJS("document.execCommand('selectAll')")
	})

	win.SetMenu(m)
	_ = wailsApp
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
