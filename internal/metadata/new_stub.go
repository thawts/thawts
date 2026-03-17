//go:build !darwin

package metadata

// New returns the default (stub) metadata provider.
// Use build tag -tags metadata_cgo on macOS to get the real implementation.
func New() Provider { return NewStubProvider() }
