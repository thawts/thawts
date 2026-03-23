//go:build !darwin

package main

import thawtsapp "github.com/thawts/thawts/internal/app"

func registerReviewHotkey(_ *thawtsapp.App, _ string) func(string) {
	return func(_ string) {} // no-op on non-macOS
}
