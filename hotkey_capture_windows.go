//go:build windows

package main

import (
	"log"
	"runtime"
	"syscall"
	"unsafe"

	"github.com/wailsapp/wails/v3/pkg/application"

	thawtsapp "github.com/thawts/thawts/internal/app"
)

func registerCaptureHotkey(app *thawtsapp.App) {
	go func() {
		runtime.LockOSThread()
		// Note: do NOT call runtime.UnlockOSThread — this goroutine runs for
		// the lifetime of the app and must stay on the same OS thread so that
		// WM_HOTKEY messages are delivered to the correct thread queue.

		user32 := syscall.NewLazyDLL("user32.dll")
		registerHotKeyProc := user32.NewProc("RegisterHotKey")
		getMessageProc := user32.NewProc("GetMessageW")

		const (
			modCtrl  uintptr = 0x0002
			modShift uintptr = 0x0004
			vkSpace  uintptr = 0x0020
			wmHotkey uint32  = 0x0312
			hotkeyID uintptr = 1
		)

		ret, _, err := registerHotKeyProc.Call(0, hotkeyID, modCtrl|modShift, vkSpace)
		if ret == 0 {
			log.Printf("WARNING: global hotkey registration failed: %v", err)
			log.Printf("         Ctrl+Shift+Space will not work. Use the system tray icon to open the capture window.")
			return
		}
		log.Printf("global hotkey Ctrl+Shift+Space registered successfully")

		type winMsg struct {
			Hwnd    uintptr
			Message uint32
			WParam  uintptr
			LParam  uintptr
			Time    uint32
			PtX     int32
			PtY     int32
		}

		var m winMsg
		for {
			ret, _, _ := getMessageProc.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
			if ret == 0 || ret == ^uintptr(0) { // WM_QUIT or error
				return
			}
			if m.Message == wmHotkey && m.WParam == hotkeyID {
				log.Printf("hotkey keydown received")
				application.InvokeSync(app.ToggleCapture)
				log.Printf("hotkey handler returned")
			}
		}
	}()
}
