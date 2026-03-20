//go:build !darwin

package main

import thawtsapp "github.com/thawts/thawts/internal/app"

func registerReviewHotkey(_ *thawtsapp.App) {
	// Cmd+Option+R is macOS-only; no equivalent on other platforms
}
