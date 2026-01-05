//go:build windows

package tray

import (
	"thawts-client/internal/app"
	"thawts-client/internal/icon"

	"github.com/getlantern/systray"
)

func RegisterApp(appInstance *app.App) {
	globalApp = appInstance
}

var globalApp *app.App

func InitTray() {
	// systray.Run blocks, so we run it in a goroutine
	go systray.Run(onReady, onExit)
}

func onReady() {
	systray.SetIcon(icon.Data)
	systray.SetTitle("Thawts")
	systray.SetTooltip("Thawts")

	mShow := systray.AddMenuItem("Show Thawts", "Show the main window")
	systray.AddSeparator()
	mExport := systray.AddMenuItem("Export Data...", "Export your thoughts")
	mImport := systray.AddMenuItem("Import Data...", "Import thoughts")
	systray.AddSeparator()
	mQuit := systray.AddMenuItem("Quit", "Quit the application")

	go func() {
		for {
			select {
			case <-mShow.ClickedCh:
				if globalApp != nil {
					globalApp.Show()
				}
			case <-mExport.ClickedCh:
				if globalApp != nil {
					globalApp.ExportThoughts()
				}
			case <-mImport.ClickedCh:
				if globalApp != nil {
					globalApp.ImportThoughts()
				}
			case <-mQuit.ClickedCh:
				systray.Quit()
			}
		}
	}()
}

func onExit() {
	if globalApp != nil {
		globalApp.Quit()
	}
}
