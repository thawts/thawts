//go:build darwin && !metadata_cgo

package metadata

import (
	"os/exec"
	"strings"
)

// darwinProvider captures metadata using osascript (no CGO required).
// For the CGO implementation using NSWorkspace + CGWindowList, build with -tags metadata_cgo.
type darwinProvider struct{}

// New returns the osascript-based macOS metadata provider.
func New() Provider { return &darwinProvider{} }

func (p *darwinProvider) GetActiveAppName() string {
	out, err := exec.Command("osascript", "-e",
		`tell application "System Events" to name of first process where it is frontmost`,
	).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// GetActiveWindowTitle is not available without Accessibility permissions.
func (p *darwinProvider) GetActiveWindowTitle() string { return "" }

// GetActiveURL is not implemented in the script provider.
func (p *darwinProvider) GetActiveURL() string { return "" }
