// Package metadata captures the execution context when a thought is entered.
//
// The capture is OS-specific (requires Accessibility permissions on macOS,
// UIAutomation on Windows). The default implementation is a no-op stub.
// Platform-specific implementations can be added without changing the interface.
package metadata

// Provider returns context about the currently active application.
type Provider interface {
	// GetActiveWindowTitle returns the title of the currently focused window.
	GetActiveWindowTitle() string

	// GetActiveAppName returns the name of the currently active application.
	GetActiveAppName() string

	// GetActiveURL returns the URL if the active app is a browser, or empty.
	GetActiveURL() string
}
