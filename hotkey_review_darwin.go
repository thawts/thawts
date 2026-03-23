//go:build darwin

package main

import (
	thawtsapp "github.com/thawts/thawts/internal/app"
)

func registerReviewHotkey(app *thawtsapp.App, hotkeyStr string) func(string) {
	slot := newHotkeySlot(app.ShowReview)
	if hotkeyStr != "" {
		slot.update(hotkeyStr)
	}
	return slot.update
}
