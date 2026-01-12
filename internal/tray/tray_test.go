package tray

import (
	"context"
	"path/filepath"
	"testing"
	"thawts-client/internal/app"
	"thawts-client/internal/storage"
)

func TestRegisterApp(t *testing.T) {
	// This test just ensures RegisterApp sets the global variable without panic.
	// We cannot test InitTray easily as it involves CGO/UI loops.

	// Init Storage
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_tray.db")
	store, err := storage.NewService(dbPath)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer store.Close()

	// Create a dummy app
	a := app.NewApp(store)
	a.Startup(context.Background())

	// Call RegisterApp
	RegisterApp(a)

	if globalApp != a {
		t.Error("RegisterApp did not set globalApp correctly")
	}
}
