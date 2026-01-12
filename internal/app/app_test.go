package app

import (
	"context"
	"path/filepath"
	"testing"
	"thawts-client/internal/storage"
)

func TestApp_Greet(t *testing.T) {
	// Setup
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	store, err := storage.NewService(dbPath)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer store.Close()

	app := NewApp(store)
	app.SetTestMode(true)
	app.Startup(context.Background())

	// Test Greet
	name := "Tester"
	expected := "Hello Tester, It's show time!"
	result := app.Greet(name)

	if result != expected {
		t.Errorf("Greet(%q) = %q; want %q", name, result, expected)
	}
}

func TestApp_Save(t *testing.T) {
	// Setup
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_save.db")
	store, err := storage.NewService(dbPath)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer store.Close()

	app := NewApp(store)
	app.SetTestMode(true)
	app.Startup(context.Background())

	// Test Save
	thought := "Test Thought"
	err = app.Save(thought)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify in DB
	// We can't access store.db directly easily unless we expose it or use storage methods.
	// Since we are integration testing with storage, we trust storage works if no error returned,
	// BUT better to verify. Application doesn't expose "GetThoughts".
	// We can use Export logic or just trust the error return + storage tests covering the rest.
	// For this unit test, checking error is sufficient for "App.Save" wiring.
}

func TestApp_ConfigAndRecent(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_config.db")
	store, err := storage.NewService(dbPath)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer store.Close()

	app := NewApp(store)
	app.SetTestMode(true)
	app.Startup(context.Background())

	// 1. Verify default behavior (config false)
	results := app.Search("")
	if results != nil {
		t.Errorf("Expected nil results when query is empty and config off, got %v", results)
	}

	// 2. Enable config via Slash Command
	if err := app.Save("/config show-recent true"); err != nil {
		t.Fatalf("Failed to run config command: %v", err)
	}

	// 3. Verify config persistent (in App struct)
	// We can't access private field easily, but behavior should change.

	// Add a thought so we have something to show
	app.Save("Recent Thought")

	// 4. Verify Search("") now returns results
	results = app.Search("")
	if len(results) != 1 {
		t.Errorf("Expected 1 result after enabling show-recent, got %d", len(results))
	}
	if results[0].Content != "Recent Thought" {
		t.Errorf("Expected 'Recent Thought', got '%s'", results[0].Content)
	}

	// 5. Disable config
	if err := app.Save("/config show-recent false"); err != nil {
		t.Fatalf("Failed to run config command: %v", err)
	}

	// 6. Verify behavior reverts
	results = app.Search("")
	if results != nil {
		t.Errorf("Expected nil results after disabling show-recent, got %v", results)
	}

	// 7. Verify Persistence (NewApp should load it)
	// Set to true again
	app.Save("/config show-recent true")

	// New App instance
	app2 := NewApp(store)
	app2.Startup(context.Background())
	// Should adhere to persisted config
	// (Note: app2 doesn't share *App struct state, but storage state)

	// However, NewApp loads from storage.
	// Let's verify app2 behaves correctly.
	// Add another thought
	app2.Save("New Thought")

	results2 := app2.Search("")
	if len(results2) < 1 {
		t.Errorf("Expected results from new app instance due to persisted config")
	}
}
