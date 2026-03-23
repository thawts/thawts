//go:build !windows

package main

import (
	"log"
	"sync"

	"golang.design/x/hotkey"
)

// hotkeySlot manages a single global hotkey that can be updated at runtime.
// Call update() from any goroutine to swap the hotkey; the running goroutine
// will unregister the old one and exit, then a new goroutine registers the new one.
type hotkeySlot struct {
	mu      sync.Mutex
	current string
	stopCh  chan struct{}
	fn      func()
}

func newHotkeySlot(fn func()) *hotkeySlot {
	return &hotkeySlot{fn: fn}
}

func (s *hotkeySlot) update(str string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if str == s.current {
		return
	}

	// Stop the previous goroutine; it will unregister its hotkey via defer.
	if s.stopCh != nil {
		close(s.stopCh)
		s.stopCh = nil
	}

	s.current = str

	if str == "" {
		return
	}

	mods, key := parseHotkeyString(str)
	if key == 0 {
		log.Printf("WARNING: invalid hotkey %q — skipping registration", str)
		return
	}

	stopCh := make(chan struct{})
	s.stopCh = stopCh
	fn := s.fn
	label := str

	// Carbon's RegisterEventHotKey must NOT be called from the Go main goroutine
	// on darwin — running in a goroutine satisfies this on all unix platforms.
	go func() {
		hk := hotkey.New(mods, key)
		if err := hk.Register(); err != nil {
			log.Printf("WARNING: hotkey registration failed (%s): %v", label, err)
			return
		}
		log.Printf("hotkey registered: %s", label)
		defer func() {
			if err := hk.Unregister(); err != nil {
				log.Printf("hotkey unregister (%s): %v", label, err)
			}
		}()

		kd := hk.Keydown()
		for {
			select {
			case <-stopCh:
				return
			case _, ok := <-kd:
				if !ok {
					return
				}
				fn()
			}
		}
	}()
}
