//go:build darwin

package main

import (
	"github.com/wailsapp/wails/v3/pkg/application"

	thawtsapp "github.com/thawts/thawts/internal/app"
)

func registerCaptureHotkey(app *thawtsapp.App, hotkeyStr string) func(string) {
	slot := newHotkeySlot(func() { application.InvokeSync(app.ToggleCapture) })
	slot.update(hotkeyStr)
	return slot.update
}
