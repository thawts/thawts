//go:build windows

package main

import (
	"log"
	"runtime"
	"strings"
	"syscall"
	"unsafe"
)

const (
	winModNoRepeat uintptr = 0x4000
	winHotkeyID    uintptr = 1
	wmHotkey       uint32  = 0x0312
	wmReconfigHK   uint32  = 0x8000 // WM_APP — reconfigure hotkey
)

// windowsHotkeySlot manages a single Win32 global hotkey on a dedicated locked OS thread.
// The thread runs a Win32 message loop. Calling update() posts WM_APP to the thread,
// which unregisters the old hotkey and registers the new one without restarting the loop.
type windowsHotkeySlot struct {
	current  string
	readyCh  chan struct{}
	threadID uint32

	postThreadMessage *syscall.LazyProc
	unregisterHotKey  *syscall.LazyProc
	registerHotKey    *syscall.LazyProc
}

func newWindowsHotkeySlot(fn func(), initialStr string) *windowsHotkeySlot {
	user32 := syscall.NewLazyDLL("user32.dll")
	s := &windowsHotkeySlot{
		current:           initialStr,
		readyCh:           make(chan struct{}),
		postThreadMessage: user32.NewProc("PostThreadMessageW"),
		unregisterHotKey:  user32.NewProc("UnregisterHotKey"),
		registerHotKey:    user32.NewProc("RegisterHotKey"),
	}

	mods, vk := parseWindowsHotkey(initialStr)

	go func() {
		runtime.LockOSThread()
		// Do NOT call UnlockOSThread: this goroutine owns the message queue
		// for the lifetime of the app.

		getMessageProc := user32.NewProc("GetMessageW")
		getCurrentThreadId := user32.NewProc("GetCurrentThreadId")

		tid, _, _ := getCurrentThreadId.Call()
		s.threadID = uint32(tid)
		close(s.readyCh) // threadID is now readable

		if vk != 0 {
			ret, _, err := s.registerHotKey.Call(0, winHotkeyID, mods|winModNoRepeat, vk)
			if ret == 0 {
				log.Printf("WARNING: hotkey registration failed: %v", err)
			} else {
				log.Printf("hotkey registered: %s", initialStr)
			}
		}

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
			switch m.Message {
			case wmHotkey:
				if m.WParam == winHotkeyID {
					fn()
				}
			case wmReconfigHK:
				s.unregisterHotKey.Call(0, winHotkeyID)
				newMods := m.WParam
				newVk := m.LParam
				if newVk != 0 {
					ret, _, err := s.registerHotKey.Call(0, winHotkeyID, newMods|winModNoRepeat, newVk)
					if ret == 0 {
						log.Printf("WARNING: hotkey re-registration failed: %v", err)
					}
				}
			}
		}
	}()

	return s
}

func (s *windowsHotkeySlot) update(str string) {
	if str == s.current {
		return
	}
	s.current = str
	<-s.readyCh // ensure thread is ready (no-op after first call)
	mods, vk := parseWindowsHotkey(str)
	s.postThreadMessage.Call(uintptr(s.threadID), uintptr(wmReconfigHK), mods, vk)
}

// parseWindowsHotkey converts a "+" separated hotkey string to Win32 modifier flags and virtual key code.
func parseWindowsHotkey(s string) (mods uintptr, vk uintptr) {
	const (
		winModAlt   uintptr = 0x0001
		winModCtrl  uintptr = 0x0002
		winModShift uintptr = 0x0004
		winModWin   uintptr = 0x0008
	)
	for _, part := range strings.Split(strings.ToLower(strings.TrimSpace(s)), "+") {
		part = strings.TrimSpace(part)
		switch part {
		case "ctrl":
			mods |= winModCtrl
		case "alt", "option":
			mods |= winModAlt
		case "shift":
			mods |= winModShift
		case "cmd", "win", "super":
			mods |= winModWin
		case "space":
			vk = 0x0020
		default:
			if len(part) == 1 && part[0] >= 'a' && part[0] <= 'z' {
				vk = uintptr(part[0]-'a') + 0x41 // VK_A = 0x41
			}
		}
	}
	return
}
