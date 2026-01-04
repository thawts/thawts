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
	if err := s.SaveThought("Hello World"); err != nil {
		t.Fatalf("SaveThought failed: %v", err)
	}
	if err := s.SaveThought("Second Thought"); err != nil {
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
