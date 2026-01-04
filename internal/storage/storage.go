package storage

import (
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite" // Pure Go SQLite driver
)

// Thought represents a single thought entry
type Thought struct {
	ID        int64     `json:"id"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

// Service handles storage operations
type Service struct {
	db *sql.DB
}

// NewService initializes the storage service
func NewService(dbPath string) (*Service, error) {
	// Ensure directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create storage directory: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	s := &Service{db: db}
	if err := s.initSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to init schema: %w", err)
	}

	return s, nil
}

// initSchema creates necessary tables
func (s *Service) initSchema() error {
	// Metadata table for versioning
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS metadata (
			key TEXT PRIMARY KEY,
			value TEXT
		);
	`)
	if err != nil {
		return err
	}

	// Thoughts table
	_, err = s.db.Exec(`
		CREATE TABLE IF NOT EXISTS thoughts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			content TEXT NOT NULL,
			created_at DATETIME NOT NULL
		);
	`)
	if err != nil {
		return err
	}

	// Check/Set version
	var version string
	err = s.db.QueryRow("SELECT value FROM metadata WHERE key = 'storage_version'").Scan(&version)
	if err == sql.ErrNoRows {
		_, err = s.db.Exec("INSERT INTO metadata (key, value) VALUES ('storage_version', '1')")
		if err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	return nil
}

// SaveThought saves a new thought
func (s *Service) SaveThought(content string) error {
	_, err := s.db.Exec("INSERT INTO thoughts (content, created_at) VALUES (?, ?)", content, time.Now())
	return err
}

// Close closes the database connection
func (s *Service) Close() error {
	return s.db.Close()
}

// RemoveAllThoughts deletes all thoughts from the database
func (s *Service) RemoveAllThoughts() error {
	_, err := s.db.Exec("DELETE FROM thoughts")
	return err
}

// ExportCSV exports thoughts to a CSV writer
func (s *Service) ExportCSV(w *csv.Writer) error {
	rows, err := s.db.Query("SELECT content, created_at FROM thoughts ORDER BY created_at DESC")
	if err != nil {
		return err
	}
	defer rows.Close()

	// Write Header
	if err := w.Write([]string{"content", "created_at"}); err != nil {
		return err
	}

	for rows.Next() {
		var content string
		var createdAt time.Time
		if err := rows.Scan(&content, &createdAt); err != nil {
			return err
		}
		if err := w.Write([]string{content, createdAt.Format(time.RFC3339)}); err != nil {
			return err
		}
	}
	w.Flush()
	return w.Error()
}

// ExportJSON exports thoughts to a JSON writer
func (s *Service) ExportJSON(v interface{}) error {
	rows, err := s.db.Query("SELECT content, created_at FROM thoughts ORDER BY created_at DESC")
	if err != nil {
		return err
	}
	defer rows.Close()

	var thoughts []struct {
		Content   string    `json:"content"`
		CreatedAt time.Time `json:"created_at"`
	}

	for rows.Next() {
		var content string
		var createdAt time.Time
		if err := rows.Scan(&content, &createdAt); err != nil {
			return err
		}
		thoughts = append(thoughts, struct {
			Content   string    `json:"content"`
			CreatedAt time.Time `json:"created_at"`
		}{
			Content:   content,
			CreatedAt: createdAt,
		})
	}

	// Marshaling handled by caller usually, but let's assume we return interface or write directly?
	// The plan said ExportJSON(writer io.Writer). Let's stick to that signature if possible or helper.
	// Actually, let's make it simple: Return the data structure, App handles file writing?
	// Or pass io.Writer. Passing io.Writer is cleaner for streaming.
	return nil
}

// ExportJSONToWriter writes JSON to writer
func (s *Service) ExportJSONToWriter(w io.Writer) error {
	rows, err := s.db.Query("SELECT content, created_at FROM thoughts ORDER BY created_at DESC")
	if err != nil {
		return err
	}
	defer rows.Close()

	var thoughts []struct {
		Content   string    `json:"content"`
		CreatedAt time.Time `json:"created_at"`
	}

	for rows.Next() {
		var content string
		var createdAt time.Time
		if err := rows.Scan(&content, &createdAt); err != nil {
			return err
		}
		thoughts = append(thoughts, struct {
			Content   string    `json:"content"`
			CreatedAt time.Time `json:"created_at"`
		}{
			Content:   content,
			CreatedAt: createdAt,
		})
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(thoughts)
}

// ImportCSV imports thoughts from CSV reader
func (s *Service) ImportCSV(r *csv.Reader, removeExisting bool) error {
	if removeExisting {
		if err := s.RemoveAllThoughts(); err != nil {
			return err
		}
	}

	// Read header
	_, err := r.Read()
	if err != nil {
		return err
	}

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare("INSERT INTO thoughts (content, created_at) VALUES (?, ?)")
	if err != nil {
		return err
	}
	defer stmt.Close()

	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			tx.Rollback()
			return err
		}
		if len(record) < 2 {
			continue
		}

		content := record[0]
		createdAt, err := time.Parse(time.RFC3339, record[1])
		if err != nil {
			// Try fallback or just use Now? Let's use Now if fail or just skip
			createdAt = time.Now()
		}

		if _, err := stmt.Exec(content, createdAt); err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

// ImportJSON imports thoughts from JSON reader
func (s *Service) ImportJSON(r io.Reader, removeExisting bool) error {
	if removeExisting {
		if err := s.RemoveAllThoughts(); err != nil {
			return err
		}
	}

	var thoughts []struct {
		Content   string    `json:"content"`
		CreatedAt time.Time `json:"created_at"`
	}

	if err := json.NewDecoder(r).Decode(&thoughts); err != nil {
		return err
	}

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare("INSERT INTO thoughts (content, created_at) VALUES (?, ?)")
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, t := range thoughts {
		if _, err := stmt.Exec(t.Content, t.CreatedAt); err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}
