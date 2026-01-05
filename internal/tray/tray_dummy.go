//go:build !darwin && !windows

package tray

import "thawts-client/internal/app"

func RegisterApp(app *app.App) {
	// No-op
}

func InitTray() {
	// No-op
}
