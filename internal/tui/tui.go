// Package tui provides a terminal user interface for Thawts.
//
// Launch with: thawts --tui
// Same service layer as the Wails GUI; no window system required.
package tui

import (
	"fmt"
	"log"

	tea "github.com/charmbracelet/bubbletea"

	"thawts-client/internal/ai"
	"thawts-client/internal/metadata"
	"thawts-client/internal/service"
	"thawts-client/internal/storage"
)

// Run initialises the service, wires a TUINotifier, and starts the Bubble Tea
// program in full-screen alternate buffer mode.
func Run(store storage.Storage, aiProvider ai.Provider, metaProvider metadata.Provider) error {
	// We need the program handle before creating the notifier, so we create
	// the service with a noop notifier first, then swap it out via the
	// TUINotifier once the program is created.
	notifier := &TUINotifier{}
	svc := service.New(store, aiProvider, metaProvider, notifier)

	m := newModel(svc)
	p := tea.NewProgram(m, tea.WithAltScreen())
	notifier.program = p

	if _, err := p.Run(); err != nil {
		return fmt.Errorf("tui: %w", err)
	}
	return nil
}

// TUINotifier implements service.Notifier by forwarding events into the
// running Bubble Tea program as NotifyMsg messages.
type TUINotifier struct {
	program *tea.Program
}

func (n *TUINotifier) Emit(event string, data ...any) {
	if n.program == nil {
		log.Printf("TUINotifier: program not set, dropping event %q", event)
		return
	}
	n.program.Send(NotifyMsg{Event: event, Data: data})
}
