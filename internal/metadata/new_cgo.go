//go:build darwin && metadata_cgo

package metadata

// New returns the macOS CGO metadata provider when built with -tags metadata_cgo.
func New() Provider { return NewMacOSProvider() }
