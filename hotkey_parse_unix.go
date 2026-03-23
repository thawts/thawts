//go:build !windows

package main

import (
	"strings"

	"golang.design/x/hotkey"
)

// parseHotkeyString converts a "+" separated hotkey string (e.g. "ctrl+option+space")
// into modifier flags and a key constant for the golang.design/x/hotkey library.
// Returns nil mods and 0 key on any parse error.
func parseHotkeyString(s string) ([]hotkey.Modifier, hotkey.Key) {
	parts := strings.Split(strings.ToLower(strings.TrimSpace(s)), "+")
	var mods []hotkey.Modifier
	var key hotkey.Key
	for _, part := range parts {
		part = strings.TrimSpace(part)
		switch part {
		case "ctrl":
			mods = append(mods, hotkey.ModCtrl)
		case "shift":
			mods = append(mods, hotkey.ModShift)
		case "alt", "option":
			mods = append(mods, hotkeyModAlt)
		case "cmd", "command", "super":
			mods = append(mods, hotkeyModCmd)
		default:
			key = parseKey(part)
		}
	}
	return mods, key
}

func parseKey(s string) hotkey.Key {
	switch s {
	case "space":
		return hotkey.KeySpace
	case "a":
		return hotkey.KeyA
	case "b":
		return hotkey.KeyB
	case "c":
		return hotkey.KeyC
	case "d":
		return hotkey.KeyD
	case "e":
		return hotkey.KeyE
	case "f":
		return hotkey.KeyF
	case "g":
		return hotkey.KeyG
	case "h":
		return hotkey.KeyH
	case "i":
		return hotkey.KeyI
	case "j":
		return hotkey.KeyJ
	case "k":
		return hotkey.KeyK
	case "l":
		return hotkey.KeyL
	case "m":
		return hotkey.KeyM
	case "n":
		return hotkey.KeyN
	case "o":
		return hotkey.KeyO
	case "p":
		return hotkey.KeyP
	case "q":
		return hotkey.KeyQ
	case "r":
		return hotkey.KeyR
	case "s":
		return hotkey.KeyS
	case "t":
		return hotkey.KeyT
	case "u":
		return hotkey.KeyU
	case "v":
		return hotkey.KeyV
	case "w":
		return hotkey.KeyW
	case "x":
		return hotkey.KeyX
	case "y":
		return hotkey.KeyY
	case "z":
		return hotkey.KeyZ
	}
	return 0
}
