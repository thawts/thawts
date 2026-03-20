//go:build darwin

package main

import (
	"golang.design/x/hotkey"

	thawtsapp "github.com/thawts/thawts/internal/app"
)

func registerReviewHotkey(app *thawtsapp.App) {
	// Cmd+Option+R — macOS only
	go registerHotkey(
		[]hotkey.Modifier{hotkey.ModCmd, hotkey.ModOption},
		hotkey.KeyR,
		app.ShowReview,
	)
}
