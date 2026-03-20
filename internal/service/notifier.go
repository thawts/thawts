package service

import "sync"

// Notifier abstracts event emission so Service has no direct dependency on any
// UI framework. Each runtime mode (Wails, TUI, web) supplies its own implementation.
type Notifier interface {
	Emit(event string, data ...any)
}

// NoopNotifier silently drops all events. Use in tests and the TUI until a real
// TUINotifier is wired.
type NoopNotifier struct{}

func (n *NoopNotifier) Emit(_ string, _ ...any) {}

// RecordingNotifier captures emitted events. Use in tests to assert that the
// service emits the expected events at the expected times.
type RecordingNotifier struct {
	mu     sync.Mutex
	events []EmittedEvent
}

// EmittedEvent holds a single captured emission.
type EmittedEvent struct {
	Name string
	Data []any
}

func (r *RecordingNotifier) Emit(event string, data ...any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, EmittedEvent{Name: event, Data: data})
}

// Events returns a snapshot of all emitted events in order.
func (r *RecordingNotifier) Events() []EmittedEvent {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]EmittedEvent, len(r.events))
	copy(out, r.events)
	return out
}

// HasEvent reports whether an event with the given name was emitted at least once.
func (r *RecordingNotifier) HasEvent(name string) bool {
	for _, e := range r.Events() {
		if e.Name == name {
			return true
		}
	}
	return false
}
