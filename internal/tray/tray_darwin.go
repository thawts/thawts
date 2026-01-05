//go:build darwin

package tray

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa
#include "tray_impl.h"
*/
import "C"
import (
	"thawts-client/internal/app"
)

var globalApp *app.App

// RegisterApp saves the app instance for callbacks
func RegisterApp(app *app.App) {
	globalApp = app
}

// InitTray initializes the tray
func InitTray() {
	C.setupTray()
}

//export handleTrayClick
func handleTrayClick(itemID C.int) {
	if globalApp == nil {
		return
	}
	switch itemID {
	case 1: // Show
		go globalApp.Show()
	case 2: // Quit
		go globalApp.Quit()
	case 3: // Export
		go globalApp.ExportThoughts()
	case 4: // Import
		go globalApp.ImportThoughts()
	}
}
