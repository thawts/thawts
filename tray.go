package main

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa
#include "tray_impl.h"
*/
import "C"

var globalApp *App

// RegisterApp saves the app instance for callbacks
func RegisterApp(app *App) {
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
