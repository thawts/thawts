package main

import (
	"embed"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"slices"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	thawtsapp "github.com/thawts/thawts/internal/app"
	"github.com/thawts/thawts/internal/ai/onnx"
	"github.com/thawts/thawts/internal/install"
	"github.com/thawts/thawts/internal/metadata"
	"github.com/thawts/thawts/internal/service"
	"github.com/thawts/thawts/internal/storage"
	"github.com/thawts/thawts/internal/tray"
	"github.com/thawts/thawts/internal/tui"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/appicon.png
var appIcon []byte

// version is set by GoReleaser at build time via -ldflags.
var version = "dev"

func main() {
	if slices.Contains(os.Args[1:], "--version") {
		fmt.Println("thawts", version)
		return
	}

	if slices.Contains(os.Args[1:], "--install") {
		execPath, err := executablePath()
		if err != nil {
			log.Fatal(err)
		}
		if err := install.Register(execPath); err != nil {
			log.Fatal("install failed: ", err)
		}
		fmt.Println("thawts will now start automatically on login.")
		return
	}

	if slices.Contains(os.Args[1:], "--uninstall") {
		if err := install.Unregister(); err != nil {
			log.Fatal("uninstall failed: ", err)
		}
		fmt.Println("thawts removed from login items.")
		return
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
	}

	store, err := storage.NewSQLiteStorage(filepath.Join(homeDir, ".thawts", "thawts.db"))
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()

	// Terminal UI mode: same service layer, no Wails dependency.
	if slices.Contains(os.Args[1:], "--tui") {
		if err := tui.Run(
			store,
			onnx.NewProvider(),
			metadata.New(),
		); err != nil {
			log.Fatal(err)
		}
		return
	}

	assetsFS, err := fs.Sub(assets, "frontend/dist")
	if err != nil {
		log.Fatal(err)
	}

	// app is declared early so the SingleInstance closure can capture it.
	// It is assigned before wailsApp.Run(), so it will be set by the time the
	// callback fires.
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
					application.InvokeSync(app.ShowCapture)
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

	svc := service.New(
		store,
		onnx.NewProvider(),
		metadata.New(),
		thawtsapp.NewWailsNotifier(wailsApp),
	)

	app = thawtsapp.NewApp(wailsApp, win, svc)

	wailsApp.RegisterService(application.NewService(app))

	// Hide instead of close — keeps the app alive in the tray.
	win.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		e.Cancel()
		win.Hide()
	})

	// Hide on focus loss, but only in capture mode (review mode should stay open).
	// On Windows the focus mechanics after tray/hotkey interactions cause spurious
	// focus-loss events that hide the window immediately — disable auto-hide there
	// and rely on the hotkey toggle and Esc key instead.
	if runtime.GOOS != "windows" {
		win.RegisterHook(events.Common.WindowLostFocus, func(_ *application.WindowEvent) {
			if app.IsCapturing() && !app.IsDialogOpen() {
				app.HideWindow()
			}
		})
	}

	buildMenu(wailsApp, win, app)
	tray.Init(wailsApp, appIcon, app)

	// Load user settings so hotkeys can be customised.
	settings, err := svc.GetSettings()
	if err != nil {
		log.Printf("WARNING: could not load settings: %v", err)
	}

	// Register global hotkeys; capture the updater functions for live hotkey changes.
	captureUpdater := registerCaptureHotkey(app, settings.CaptureHotkey)
	reviewUpdater  := registerReviewHotkey(app, settings.ReviewHotkey)

	app.SetHotkeyReloader(func(capture, review string) {
		captureUpdater(capture)
		reviewUpdater(review)
	})

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

// executablePath returns the absolute path to the running binary, resolving symlinks.
// This ensures --install points to the real binary, not a Homebrew symlink.
func executablePath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(exe)
}

