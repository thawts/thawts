package app

import "github.com/wailsapp/wails/v3/pkg/application"

// WailsNotifier adapts the Wails EventManager to the service.Notifier interface.
type WailsNotifier struct {
	app *application.App
}

// NewWailsNotifier creates a Notifier backed by the Wails event system.
func NewWailsNotifier(app *application.App) *WailsNotifier {
	return &WailsNotifier{app: app}
}

func (n *WailsNotifier) Emit(event string, data ...any) {
	n.app.Event.Emit(event, data...)
}
