//go:build windows

package main

import (
	"github.com/wailsapp/wails/v3/pkg/application"

	thawtsapp "github.com/thawts/thawts/internal/app"
)

func registerCaptureHotkey(app *thawtsapp.App, hotkeyStr string) func(string) {
	slot := newWindowsHotkeySlot(func() { application.InvokeSync(app.ToggleCapture) }, hotkeyStr)
	return slot.update
}
