//go:build !windows && !darwin

package main

import "golang.design/x/hotkey"

const hotkeyModAlt hotkey.Modifier = hotkey.Mod1 // Mod1 = Alt on X11/Linux
const hotkeyModCmd hotkey.Modifier = 0            // no Cmd on Linux
