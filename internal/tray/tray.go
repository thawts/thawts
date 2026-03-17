// Package tray is a stub until a proper cross-platform tray integration is added.
//
// The conflict between getlantern/systray and Wails (both define an Objective-C
// AppDelegate on macOS) means we cannot use getlantern/systray alongside Wails.
// For now the app is accessible via:
//   - Ctrl+Shift+Space — capture mode
//   - Cmd+Option+R     — review mode
//   - Dock icon + app menu bar
//
// TODO: integrate Wails' built-in TrayMenu API when macOS support stabilises.
package tray

// App is the subset of the application needed by the tray.
type App interface {
	ShowCapture()
	ShowReview()
	Quit()
}

// Init is a no-op stub. Replace with a real implementation per platform.
func Init(_ App, _ []byte) {}
