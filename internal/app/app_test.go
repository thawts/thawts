package app

// Business logic tests live in internal/service/service_test.go.
// Tests here cover only App-level behaviour (IsCapturing getter, etc.).
// Window control methods require a live Wails instance and are covered by
// manual testing.

import (
	"path/filepath"
	"testing"

	appai "thawts-client/internal/ai"
	"thawts-client/internal/metadata"
	"thawts-client/internal/service"
	"thawts-client/internal/storage"
)

func newTestApp(t *testing.T) *App {
	t.Helper()
	store, err := storage.NewSQLiteStorage(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("NewSQLiteStorage: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	svc := service.New(store, appai.NewStubProvider(), metadata.NewStubProvider(), &service.NoopNotifier{})
	// Pass nil for wailsApp and window — window methods are not called in these tests.
	return NewApp(nil, nil, svc)
}

func TestIsCapturing_defaultsFalse(t *testing.T) {
	a := newTestApp(t)
	if a.IsCapturing() {
		t.Error("expected IsCapturing() == false on a new App")
	}
}
