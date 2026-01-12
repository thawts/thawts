package storage

import (
	"bytes"
	"encoding/csv"
	"path/filepath"
	"strings"
	"testing"
)

func TestService_Workflow(t *testing.T) {
	// 1. Init
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	s, err := NewService(dbPath)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}
	defer s.Close()

	// 2. Save
	if _, err := s.SaveThought("Hello World"); err != nil {
		t.Fatalf("SaveThought failed: %v", err)
	}
	if _, err := s.SaveThought("Second Thought"); err != nil {
		t.Fatalf("SaveThought failed: %v", err)
	}

	// Verify Count (basic query)
	var count int
	err = s.db.QueryRow("SELECT COUNT(*) FROM thoughts").Scan(&count)
	if err != nil {
		t.Fatalf("Query count failed: %v", err)
	}
	if count != 2 {
		t.Errorf("Expected 2 thoughts, got %d", count)
	}

	// 3. Export JSON
	var buf bytes.Buffer
	if err := s.ExportJSONToWriter(&buf); err != nil {
		t.Fatalf("ExportJSONToWriter failed: %v", err)
	}
	jsonOutput := buf.String()
	if !strings.Contains(jsonOutput, "Hello World") {
		t.Errorf("JSON output missing content: %s", jsonOutput)
	}

	// 4. Import JSON (Clear)
	// Create new buffer with JSON data that has 1 new item
	importJSON := `[{"content":"Imported Thought","created_at":"2023-01-01T00:00:00Z"}]`
	err = s.ImportJSON(strings.NewReader(importJSON), true) // Remove existing
	if err != nil {
		t.Fatalf("ImportJSON failed: %v", err)
	}

	// Verify Count -> Should be 1
	err = s.db.QueryRow("SELECT COUNT(*) FROM thoughts").Scan(&count)
	if err != nil {
		t.Fatalf("Query count failed: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected 1 thought after clear import, got %d", count)
	}

	// Verify Content
	var content string
	err = s.db.QueryRow("SELECT content FROM thoughts LIMIT 1").Scan(&content)
	if err != nil {
		t.Fatalf("Query content failed: %v", err)
	}
	if content != "Imported Thought" {
		t.Errorf("Expected 'Imported Thought', got '%s'", content)
	}
}

func TestService_CSV_ImportExport(t *testing.T) {
	// 1. Init
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_csv.db")
	s, err := NewService(dbPath)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}
	defer s.Close()

	// 2. Save
	s.SaveThought("CSV Thought")

	// 3. Export CSV
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	if err := s.ExportCSV(w); err != nil {
		t.Fatalf("ExportCSV failed: %v", err)
	}
	csvOutput := buf.String()
	if !strings.Contains(csvOutput, "CSV Thought") {
		t.Errorf("CSV output missing content: %s", csvOutput)
	}

	// 4. Import CSV (Append)
	// CSV Header: content,created_at
	importCSV := `content,created_at
Appended Thought,2023-01-02T00:00:00Z
`
	r := csv.NewReader(strings.NewReader(importCSV))
	err = s.ImportCSV(r, false) // Append
	if err != nil {
		t.Fatalf("ImportCSV failed: %v", err)
	}

	// Verify Count -> Should be 2
	var count int
	err = s.db.QueryRow("SELECT COUNT(*) FROM thoughts").Scan(&count)
	if err != nil {
		t.Fatalf("Query count failed: %v", err)
	}
	if count != 2 {
		t.Errorf("Expected 2 thoughts (1 old + 1 new), got %d", count)
	}
}

func TestSearchThoughts(t *testing.T) {
	tmpDB := filepath.Join(t.TempDir(), "test_search.db")
	service, err := NewService(tmpDB)
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}
	defer service.Close()

	// Seed data
	thoughts := []string{
		"Hello World",
		"Another thought",
		"World is big",
		"Golang is great",
	}

	for _, thought := range thoughts {
		if _, err := service.SaveThought(thought); err != nil {
			t.Fatalf("Failed to save thought: %v", err)
		}
	}

	// Test Search "World"
	results, err := service.SearchThoughts("World")
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("Expected 2 results for 'World', got %d", len(results))
	}

	// Check content
	foundHello := false
	foundBig := false
	for _, r := range results {
		if strings.Contains(r.Content, "Hello World") {
			foundHello = true
		}
		if strings.Contains(r.Content, "World is big") {
			foundBig = true
		}
	}

	if !foundHello || !foundBig {
		t.Errorf("Search results missing expected content. Got: %v", results)
	}

	// Test case insensitive (SQLite LIKE is case insensitive by default for ASCII)
	results, err = service.SearchThoughts("world")
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("Expected 2 results for 'world' (case insensitive), got %d", len(results))
	}

	// Test no match
	results, err = service.SearchThoughts("NonExistent")
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("Expected 0 results, got %d", len(results))
	}
}

// TestGetRecentThoughts tests fetching recent thoughts
func TestGetRecentThoughts(t *testing.T) {
	tmpDB := filepath.Join(t.TempDir(), "test_recent.db")
	service, err := NewService(tmpDB)
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}
	defer service.Close()

	// Seed data
	thoughts := []string{
		"Thought 1",
		"Thought 2",
		"Thought 3",
	}
	for _, thought := range thoughts {
		if _, err := service.SaveThought(thought); err != nil {
			t.Fatalf("Failed to save thought: %v", err)
		}
	}

	// Fetch recent 2
	recent, err := service.GetRecentThoughts(2)
	if err != nil {
		t.Fatalf("GetRecentThoughts failed: %v", err)
	}

	if len(recent) != 2 {
		t.Errorf("Expected 2 recent thoughts, got %d", len(recent))
	}

	// Should be ordered by created_at DESC, so Thought 3 first
	if recent[0].Content != "Thought 3" {
		t.Errorf("Expected 'Thought 3' as first recent thought, got '%s'", recent[0].Content)
	}
	if recent[1].Content != "Thought 2" {
		t.Errorf("Expected 'Thought 2' as second recent thought, got '%s'", recent[1].Content)
	}
}

// TestMetadata tests metadata operations
func TestMetadata(t *testing.T) {
	tmpDB := filepath.Join(t.TempDir(), "test_metadata.db")
	service, err := NewService(tmpDB)
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}
	defer service.Close()

	// Test Set
	if err := service.SetMetadata("test_key", "test_value"); err != nil {
		t.Fatalf("SetMetadata failed: %v", err)
	}

	// Test Get
	val, err := service.GetMetadata("test_key")
	if err != nil {
		t.Fatalf("GetMetadata failed: %v", err)
	}
	if val != "test_value" {
		t.Errorf("Expected 'test_value', got '%s'", val)
	}

	// Test Update
	if err := service.SetMetadata("test_key", "updated_value"); err != nil {
		t.Fatalf("SetMetadata (update) failed: %v", err)
	}
	val, err = service.GetMetadata("test_key")
	if err != nil {
		t.Fatalf("GetMetadata failed: %v", err)
	}
	if val != "updated_value" {
		t.Errorf("Expected 'updated_value', got '%s'", val)
	}

	// Test Non-existent
	val, err = service.GetMetadata("missing_key")
	if err != nil {
		t.Fatalf("GetMetadata (missing) failed: %v", err)
	}
	if val != "" {
		t.Errorf("Expected empty string for missing key, got '%s'", val)
	}
}
