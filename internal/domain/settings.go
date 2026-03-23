package domain

// Settings holds user-configurable preferences persisted in the database.
type Settings struct {
	// CaptureHotkey is the global hotkey for toggling the capture window,
	// encoded as a "+" separated string, e.g. "ctrl+option+space".
	// Supported modifiers: ctrl, alt, option (=alt on macOS), cmd (macOS), shift.
	// Supported keys: space, a-z. Changes take effect immediately.
	CaptureHotkey string `json:"capture_hotkey"`

	// ReviewHotkey is the global hotkey for opening review mode (macOS only).
	// Same format as CaptureHotkey. Changes take effect immediately.
	ReviewHotkey string `json:"review_hotkey"`

	// LaunchAtLogin controls whether thawts starts automatically on login.
	// Changes take effect immediately.
	LaunchAtLogin bool `json:"launch_at_login"`
}
