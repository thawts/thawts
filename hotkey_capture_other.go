//go:build !windows

package main

import (
	"log"

	"github.com/wailsapp/wails/v3/pkg/application"
	"golang.design/x/hotkey"

	thawtsapp "github.com/thawts/thawts/internal/app"
)

func registerCaptureHotkey(app *thawtsapp.App) {
	go registerHotkey(
		[]hotkey.Modifier{hotkey.ModShift, hotkey.ModCtrl},
		hotkey.KeySpace,
		func() { application.InvokeSync(app.ToggleCapture) },
	)
}

func registerHotkey(mods []hotkey.Modifier, key hotkey.Key, fn func()) {
	hk := hotkey.New(mods, key)
	if err := hk.Register(); err != nil {
		log.Printf("WARNING: global hotkey registration failed: %v", err)
		return
	}
	log.Printf("global hotkey registered successfully")
	for range hk.Keydown() {
		fn()
	}
}
