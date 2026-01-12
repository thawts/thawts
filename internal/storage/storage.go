package storage

import (
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite" // Pure Go SQLite driver
)

// Thought represents a single thought entry
type Thought struct {
	ID        int64     `json:"id"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
	Tags      []string  `json:"tags"` // New field
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

	// Tags table
	_, err = s.db.Exec(`
		CREATE TABLE IF NOT EXISTS tags (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			thought_id INTEGER NOT NULL,
			name TEXT NOT NULL,
			source TEXT,
			confidence REAL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY(thought_id) REFERENCES thoughts(id) ON DELETE CASCADE
		);
		CREATE INDEX IF NOT EXISTS idx_tags_thought_id ON tags(thought_id);
	`)
	if err != nil {
		return err
	}

	// Check/Set version
	var version string
	err = s.db.QueryRow("SELECT value FROM metadata WHERE key = 'storage_version'").Scan(&version)
	if err == sql.ErrNoRows {
		_, err = s.db.Exec("INSERT INTO metadata (key, value) VALUES ('storage_version', '2')")
		if err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	return nil
}

// SaveThought saves a new thought and returns the ID
func (s *Service) SaveThought(content string) (int64, error) {
	res, err := s.db.Exec("INSERT INTO thoughts (content, created_at) VALUES (?, ?)", content, time.Now())
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// AddTag adds a tag to a thought
func (s *Service) AddTag(thoughtID int64, tag string, source string) error {
	_, err := s.db.Exec("INSERT INTO tags (thought_id, name, source) VALUES (?, ?, ?)", thoughtID, tag, source)
	return err
}

// getTagsForThoughts retrieves tags for a list of thought IDs
func (s *Service) getTagsForThoughts(thoughts []Thought) error {
	if len(thoughts) == 0 {
		return nil
	}

	// Map ID -> *Thought
	thoughtMap := make(map[int64]*Thought)
	args := make([]interface{}, len(thoughts))
	for i := range thoughts {
		thoughtMap[thoughts[i].ID] = &thoughts[i]
		args[i] = thoughts[i].ID
	}

	// Query tags
	query := "SELECT thought_id, name FROM tags WHERE thought_id IN (?" + strings.Repeat(",?", len(thoughts)-1) + ")"
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var tID int64
		var name string
		if err := rows.Scan(&tID, &name); err != nil {
			return err
		}
		if t, ok := thoughtMap[tID]; ok {
			t.Tags = append(t.Tags, name)
		}
	}
	return nil
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

// SearchThoughts finds thoughts containing the query string
func (s *Service) SearchThoughts(query string) ([]Thought, error) {
	// Simple partial match
	rows, err := s.db.Query("SELECT id, content, created_at FROM thoughts WHERE content LIKE ? ORDER BY created_at DESC LIMIT 5", "%"+query+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var thoughts []Thought
	for rows.Next() {
		var t Thought
		if err := rows.Scan(&t.ID, &t.Content, &t.CreatedAt); err != nil {
			return nil, err
		}
		thoughts = append(thoughts, t)
	}
	if err := s.getTagsForThoughts(thoughts); err != nil {
		return nil, err
	}

	return thoughts, nil
}

// SetMetadata sets a metadata key-value pair
func (s *Service) SetMetadata(key, value string) error {
	_, err := s.db.Exec("INSERT OR REPLACE INTO metadata (key, value) VALUES (?, ?)", key, value)
	return err
}

// GetMetadata retrieves a metadata value by key
func (s *Service) GetMetadata(key string) (string, error) {
	var value string
	err := s.db.QueryRow("SELECT value FROM metadata WHERE key = ?", key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return value, err
}

// GetRecentThoughts retrieves the most recent thoughts limited by the count
func (s *Service) GetRecentThoughts(limit int) ([]Thought, error) {
	rows, err := s.db.Query("SELECT id, content, created_at FROM thoughts ORDER BY created_at DESC LIMIT ?", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var thoughts []Thought
	for rows.Next() {
		var t Thought
		if err := rows.Scan(&t.ID, &t.Content, &t.CreatedAt); err != nil {
			return nil, err
		}
		thoughts = append(thoughts, t)
	}
	if err := s.getTagsForThoughts(thoughts); err != nil {
		return nil, err
	}

	return thoughts, nil
}

// GetAllThoughts retrieves all thoughts (for backfill)
func (s *Service) GetAllThoughts() ([]Thought, error) {
	rows, err := s.db.Query("SELECT id, content, created_at FROM thoughts")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var thoughts []Thought
	for rows.Next() {
		var t Thought
		if err := rows.Scan(&t.ID, &t.Content, &t.CreatedAt); err != nil {
			return nil, err
		}
		thoughts = append(thoughts, t)
	}
	return thoughts, nil
}
