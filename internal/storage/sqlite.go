package storage

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"thawts-client/internal/domain"

	_ "modernc.org/sqlite"
)

const timeFormat = time.RFC3339

// SQLiteStorage is the local SQLite implementation of Storage.
type SQLiteStorage struct {
	db *sql.DB
}

// NewSQLiteStorage opens (or creates) the database at dbPath and runs migrations.
func NewSQLiteStorage(dbPath string) (*SQLiteStorage, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("create db dir: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// Single persistent connection so per-connection PRAGMAs (foreign_keys, WAL) are reliable.
	db.SetMaxOpenConns(1)
	db.SetConnMaxLifetime(0)

	s := &SQLiteStorage{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return s, nil
}

func (s *SQLiteStorage) migrate() error {
	stmts := []string{
		`PRAGMA journal_mode=WAL`,
		`PRAGMA foreign_keys=ON`,
		`CREATE TABLE IF NOT EXISTS thoughts (
			id           INTEGER PRIMARY KEY AUTOINCREMENT,
			content      TEXT    NOT NULL,
			raw_content  TEXT    NOT NULL DEFAULT '',
			window_title TEXT    NOT NULL DEFAULT '',
			app_name     TEXT    NOT NULL DEFAULT '',
			url          TEXT    NOT NULL DEFAULT '',
			created_at   TEXT    NOT NULL,
			updated_at   TEXT    NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS tags (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			thought_id  INTEGER NOT NULL REFERENCES thoughts(id) ON DELETE CASCADE,
			name        TEXT    NOT NULL,
			source      TEXT    NOT NULL DEFAULT 'system',
			confidence  REAL    NOT NULL DEFAULT 1.0,
			created_at  TEXT    NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_tags_thought_id ON tags(thought_id)`,
		`CREATE INDEX IF NOT EXISTS idx_thoughts_created ON thoughts(created_at)`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.Exec(stmt); err != nil {
			return fmt.Errorf("exec %q: %w", stmt[:min(40, len(stmt))], err)
		}
	}

	// Additive column migrations — safe to run on existing databases.
	alterStmts := []string{
		`ALTER TABLE thoughts ADD COLUMN raw_content  TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE thoughts ADD COLUMN window_title TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE thoughts ADD COLUMN app_name     TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE thoughts ADD COLUMN url          TEXT NOT NULL DEFAULT ''`,
	}
	for _, stmt := range alterStmts {
		if _, err := s.db.Exec(stmt); err != nil && !strings.Contains(err.Error(), "duplicate column name") {
			return fmt.Errorf("alter: %w", err)
		}
	}
	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// SaveThought implements Storage.
func (s *SQLiteStorage) SaveThought(content string, ctx domain.CaptureContext) (*domain.Thought, error) {
	now := time.Now().UTC()
	nowStr := now.Format(timeFormat)

	res, err := s.db.Exec(
		`INSERT INTO thoughts (content, raw_content, window_title, app_name, url, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		content, content,
		ctx.WindowTitle, ctx.AppName, ctx.URL,
		nowStr, nowStr,
	)
	if err != nil {
		return nil, fmt.Errorf("insert thought: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	return &domain.Thought{
		ID:         id,
		Content:    content,
		RawContent: content,
		Context:    ctx,
		Tags:       []domain.Tag{},
		CreatedAt:  now,
		UpdatedAt:  now,
	}, nil
}

// GetThought implements Storage.
func (s *SQLiteStorage) GetThought(id int64) (*domain.Thought, error) {
	t, err := s.scanOneThought(
		s.db.QueryRow(
			`SELECT id, content, raw_content, window_title, app_name, url, created_at, updated_at
			 FROM thoughts WHERE id = ?`, id,
		),
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("thought %d not found", id)
	}
	if err != nil {
		return nil, err
	}
	tagMap, err := s.fetchTagsForIDs([]int64{id})
	if err != nil {
		return nil, err
	}
	t.Tags = tagMap[id]
	if t.Tags == nil {
		t.Tags = []domain.Tag{}
	}
	return t, nil
}

// UpdateThought implements Storage.
func (s *SQLiteStorage) UpdateThought(id int64, content string) (*domain.Thought, error) {
	now := time.Now().UTC().Format(timeFormat)
	_, err := s.db.Exec(
		`UPDATE thoughts SET content = ?, updated_at = ? WHERE id = ?`,
		content, now, id,
	)
	if err != nil {
		return nil, fmt.Errorf("update thought: %w", err)
	}
	return s.GetThought(id)
}

// DeleteThought implements Storage.
func (s *SQLiteStorage) DeleteThought(id int64) error {
	_, err := s.db.Exec(`DELETE FROM thoughts WHERE id = ?`, id)
	return err
}

// SearchThoughts implements Storage.
func (s *SQLiteStorage) SearchThoughts(query string, limit int) ([]*domain.Thought, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.db.Query(
		`SELECT id, content, raw_content, window_title, app_name, url, created_at, updated_at
		 FROM thoughts
		 WHERE content LIKE ? COLLATE NOCASE
		 ORDER BY created_at DESC
		 LIMIT ?`,
		"%"+query+"%", limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return s.scanThoughts(rows)
}

// GetRecentThoughts implements Storage.
func (s *SQLiteStorage) GetRecentThoughts(limit int) ([]*domain.Thought, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.db.Query(
		`SELECT id, content, raw_content, window_title, app_name, url, created_at, updated_at
		 FROM thoughts
		 ORDER BY created_at DESC
		 LIMIT ?`,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return s.scanThoughts(rows)
}

// AddTag implements Storage.
func (s *SQLiteStorage) AddTag(thoughtID int64, name, source string, confidence float64) error {
	_, err := s.db.Exec(
		`INSERT INTO tags (thought_id, name, source, confidence, created_at) VALUES (?, ?, ?, ?, ?)`,
		thoughtID, name, source, confidence, time.Now().UTC().Format(timeFormat),
	)
	return err
}

// Close implements Storage.
func (s *SQLiteStorage) Close() error {
	return s.db.Close()
}

// --- helpers ---

func (s *SQLiteStorage) scanOneThought(row *sql.Row) (*domain.Thought, error) {
	var t domain.Thought
	var createdStr, updatedStr string
	err := row.Scan(
		&t.ID, &t.Content, &t.RawContent,
		&t.Context.WindowTitle, &t.Context.AppName, &t.Context.URL,
		&createdStr, &updatedStr,
	)
	if err != nil {
		return nil, err
	}
	t.CreatedAt, _ = time.Parse(timeFormat, createdStr)
	t.UpdatedAt, _ = time.Parse(timeFormat, updatedStr)
	return &t, nil
}

func (s *SQLiteStorage) scanThoughts(rows *sql.Rows) ([]*domain.Thought, error) {
	var thoughts []*domain.Thought
	var ids []int64

	for rows.Next() {
		var t domain.Thought
		var createdStr, updatedStr string
		if err := rows.Scan(
			&t.ID, &t.Content, &t.RawContent,
			&t.Context.WindowTitle, &t.Context.AppName, &t.Context.URL,
			&createdStr, &updatedStr,
		); err != nil {
			return nil, err
		}
		t.CreatedAt, _ = time.Parse(timeFormat, createdStr)
		t.UpdatedAt, _ = time.Parse(timeFormat, updatedStr)
		thoughts = append(thoughts, &t)
		ids = append(ids, t.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(thoughts) == 0 {
		return []*domain.Thought{}, nil
	}

	tagMap, err := s.fetchTagsForIDs(ids)
	if err != nil {
		return nil, err
	}
	for _, t := range thoughts {
		t.Tags = tagMap[t.ID]
		if t.Tags == nil {
			t.Tags = []domain.Tag{}
		}
	}
	return thoughts, nil
}

func (s *SQLiteStorage) fetchTagsForIDs(ids []int64) (map[int64][]domain.Tag, error) {
	placeholders := strings.Repeat("?,", len(ids))
	placeholders = placeholders[:len(placeholders)-1] // trim trailing comma

	args := make([]interface{}, len(ids))
	for i, id := range ids {
		args[i] = id
	}

	rows, err := s.db.Query(
		`SELECT id, thought_id, name, source, confidence, created_at
		 FROM tags WHERE thought_id IN (`+placeholders+`)`,
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[int64][]domain.Tag)
	for rows.Next() {
		var tag domain.Tag
		var createdStr string
		if err := rows.Scan(&tag.ID, &tag.ThoughtID, &tag.Name, &tag.Source, &tag.Confidence, &createdStr); err != nil {
			return nil, err
		}
		tag.CreatedAt, _ = time.Parse(timeFormat, createdStr)
		result[tag.ThoughtID] = append(result[tag.ThoughtID], tag)
	}
	return result, rows.Err()
}
