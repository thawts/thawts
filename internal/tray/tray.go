// Package tray provides native system tray support via the Wails v3 SystemTray API.
// This replaces the previous no-op stub — Wails v3's unified tray implementation
// works on macOS, Windows, and Linux without the AppDelegate conflict that
// prevented getlantern/systray from working alongside Wails v2.
package tray

import (
	"github.com/wailsapp/wails/v3/pkg/application"
)

// App is the subset of the application needed by the tray.
type App interface {
	ShowCapture()
	ShowReview()
	Quit()
}

// Init creates the native system tray icon with a context menu.
func Init(wailsApp *application.App, icon []byte, appInstance App) {
	t := wailsApp.SystemTray.New()
	t.SetIcon(icon)
	t.SetTooltip("Thawts")

	menu := application.NewMenu()
	menu.Add("Capture Thought").OnClick(func(*application.Context) {
		appInstance.ShowCapture()
	})
	menu.Add("Review Garden").OnClick(func(*application.Context) {
		appInstance.ShowReview()
	})
	menu.AddSeparator()
	menu.Add("Quit").OnClick(func(*application.Context) {
		appInstance.Quit()
	})

	t.SetMenu(menu)
	t.OnClick(func() {
		appInstance.ShowCapture()
	})
}
