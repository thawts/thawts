package storage

import (
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/thawts/thawts/internal/domain"

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

	// Embeddings table for vector similarity search.
	if _, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS embeddings (
		thought_id INTEGER PRIMARY KEY REFERENCES thoughts(id) ON DELETE CASCADE,
		vector     BLOB NOT NULL
	)`); err != nil {
		return fmt.Errorf("create embeddings table: %w", err)
	}

	// Intent table — stores actionable items extracted from thoughts.
	if _, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS intents (
		id         TEXT    PRIMARY KEY,
		thought_id INTEGER NOT NULL REFERENCES thoughts(id) ON DELETE CASCADE,
		type       TEXT    NOT NULL,
		title      TEXT    NOT NULL,
		date       TEXT,
		status     TEXT    NOT NULL DEFAULT 'pending',
		created_at TEXT    NOT NULL
	)`); err != nil {
		return fmt.Errorf("create intents table: %w", err)
	}

	// Settings table — key-value store for user preferences.
	if _, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS settings (
		key   TEXT PRIMARY KEY,
		value TEXT NOT NULL
	)`); err != nil {
		return fmt.Errorf("create settings table: %w", err)
	}

	// Wellbeing signals table — one sentiment score per thought.
	if _, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS wellbeing_signals (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		thought_id INTEGER NOT NULL REFERENCES thoughts(id) ON DELETE CASCADE,
		score      REAL    NOT NULL,
		created_at TEXT    NOT NULL
	)`); err != nil {
		return fmt.Errorf("create wellbeing_signals table: %w", err)
	}

	// Additive column migrations — safe to run on existing databases.
	alterStmts := []string{
		`ALTER TABLE thoughts ADD COLUMN raw_content  TEXT    NOT NULL DEFAULT ''`,
		`ALTER TABLE thoughts ADD COLUMN window_title TEXT    NOT NULL DEFAULT ''`,
		`ALTER TABLE thoughts ADD COLUMN app_name     TEXT    NOT NULL DEFAULT ''`,
		`ALTER TABLE thoughts ADD COLUMN url          TEXT    NOT NULL DEFAULT ''`,
		`ALTER TABLE thoughts ADD COLUMN hidden       INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE thoughts ADD COLUMN meta         TEXT`,
		`ALTER TABLE thoughts ADD COLUMN updated_at   TEXT    NOT NULL DEFAULT ''`,
	}
	for _, stmt := range alterStmts {
		if _, err := s.db.Exec(stmt); err != nil && !strings.Contains(err.Error(), "duplicate column name") {
			return fmt.Errorf("alter: %w", err)
		}
	}
	return nil
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
			`SELECT id, content, raw_content, window_title, app_name, url, hidden, meta, created_at, updated_at
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
		`SELECT id, content, raw_content, window_title, app_name, url, hidden, meta, created_at, updated_at
		 FROM thoughts
		 WHERE hidden = 0 AND content LIKE ? COLLATE NOCASE
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
		`SELECT id, content, raw_content, window_title, app_name, url, hidden, meta, created_at, updated_at
		 FROM thoughts
		 WHERE hidden = 0
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

// HideThought implements Storage.
func (s *SQLiteStorage) HideThought(id int64) error {
	_, err := s.db.Exec(`UPDATE thoughts SET hidden = 1 WHERE id = ?`, id)
	return err
}

// UnhideThought implements Storage.
func (s *SQLiteStorage) UnhideThought(id int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`UPDATE thoughts SET hidden = 0 WHERE id = ?`, id); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM tags WHERE thought_id = ? AND name = 'mishap'`, id); err != nil {
		return err
	}
	return tx.Commit()
}

// GetHiddenThoughts implements Storage.
func (s *SQLiteStorage) GetHiddenThoughts() ([]*domain.Thought, error) {
	rows, err := s.db.Query(
		`SELECT id, content, raw_content, window_title, app_name, url, hidden, meta, created_at, updated_at
		 FROM thoughts
		 WHERE hidden = 1
		 ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return s.scanThoughts(rows)
}

// StoreEmbedding implements Storage.
func (s *SQLiteStorage) StoreEmbedding(thoughtID int64, embedding []float32) error {
	if len(embedding) == 0 {
		return nil
	}
	buf := make([]byte, len(embedding)*4)
	for i, v := range embedding {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(v))
	}
	_, err := s.db.Exec(
		`INSERT INTO embeddings (thought_id, vector) VALUES (?, ?)
		 ON CONFLICT(thought_id) DO UPDATE SET vector = excluded.vector`,
		thoughtID, buf,
	)
	return err
}

// GetEmbeddings returns the stored float32 vectors for the given thought IDs.
func (s *SQLiteStorage) GetEmbeddings(ids []int64) (map[int64][]float32, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	placeholders := strings.Repeat("?,", len(ids))
	placeholders = placeholders[:len(placeholders)-1]
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	rows, err := s.db.Query(
		`SELECT thought_id, vector FROM embeddings WHERE thought_id IN (`+placeholders+`)`,
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[int64][]float32)
	for rows.Next() {
		var id int64
		var blob []byte
		if err := rows.Scan(&id, &blob); err != nil || len(blob)%4 != 0 {
			continue
		}
		vec := make([]float32, len(blob)/4)
		for i := range vec {
			vec[i] = math.Float32frombits(binary.LittleEndian.Uint32(blob[i*4:]))
		}
		result[id] = vec
	}
	return result, rows.Err()
}

// SemanticSearch implements Storage.
func (s *SQLiteStorage) SemanticSearch(query string, limit int) ([]*domain.Thought, error) {
	return s.SearchThoughts(query, limit)
}

// AddTag implements Storage.
func (s *SQLiteStorage) AddTag(thoughtID int64, name, source string, confidence float64) error {
	_, err := s.db.Exec(
		`INSERT INTO tags (thought_id, name, source, confidence, created_at) VALUES (?, ?, ?, ?, ?)`,
		thoughtID, name, source, confidence, time.Now().UTC().Format(timeFormat),
	)
	return err
}

// MergeThoughts implements Storage.
// Creates a new thought with the concatenated content of all given thoughts,
// union-merged tags, and the oldest created_at. The originals are soft-deleted.
func (s *SQLiteStorage) MergeThoughts(ids []int64) (*domain.Thought, error) {
	if len(ids) < 2 {
		return nil, fmt.Errorf("MergeThoughts requires at least 2 thought IDs")
	}

	// Load all thoughts outside the transaction.
	var thoughts []*domain.Thought
	for _, id := range ids {
		t, err := s.GetThought(id)
		if err != nil {
			return nil, fmt.Errorf("load thought %d: %w", id, err)
		}
		thoughts = append(thoughts, t)
	}

	// Find the oldest created_at.
	oldest := thoughts[0].CreatedAt
	for _, t := range thoughts[1:] {
		if t.CreatedAt.Before(oldest) {
			oldest = t.CreatedAt
		}
	}

	// Concatenate content.
	parts := make([]string, len(thoughts))
	for i, t := range thoughts {
		parts[i] = t.Content
	}
	mergedContent := strings.Join(parts, "\n\n")

	// Union tags by name.
	type tagEntry struct{ name, source string; confidence float64 }
	tagSeen := map[string]bool{}
	var uniqTags []tagEntry
	for _, t := range thoughts {
		for _, tag := range t.Tags {
			if !tagSeen[tag.Name] {
				tagSeen[tag.Name] = true
				uniqTags = append(uniqTags, tagEntry{tag.Name, tag.Source, tag.Confidence})
			}
		}
	}

	// Build meta JSON.
	metaJSON, _ := json.Marshal(map[string]any{"merged_from": ids})

	now := time.Now().UTC()
	nowStr := now.Format(timeFormat)
	oldestStr := oldest.Format(timeFormat)

	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Insert new merged thought.
	res, err := tx.Exec(
		`INSERT INTO thoughts (content, raw_content, window_title, app_name, url, meta, created_at, updated_at)
		 VALUES (?, ?, '', '', '', ?, ?, ?)`,
		mergedContent, mergedContent, string(metaJSON), oldestStr, nowStr,
	)
	if err != nil {
		return nil, fmt.Errorf("insert merged thought: %w", err)
	}
	newID, _ := res.LastInsertId()

	// Copy union-merged tags.
	for _, tag := range uniqTags {
		if _, err := tx.Exec(
			`INSERT INTO tags (thought_id, name, source, confidence, created_at) VALUES (?, ?, ?, ?, ?)`,
			newID, tag.name, tag.source, tag.confidence, nowStr,
		); err != nil {
			return nil, fmt.Errorf("insert merged tag: %w", err)
		}
	}

	// Soft-delete originals and mark them as merged.
	for _, id := range ids {
		if _, err := tx.Exec(`UPDATE thoughts SET hidden = 1 WHERE id = ?`, id); err != nil {
			return nil, fmt.Errorf("hide original %d: %w", id, err)
		}
		if _, err := tx.Exec(
			`INSERT INTO tags (thought_id, name, source, confidence, created_at) VALUES (?, 'merged', 'system', 1.0, ?)`,
			id, nowStr,
		); err != nil {
			return nil, fmt.Errorf("add merged tag to original %d: %w", id, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return s.GetThought(newID)
}

// --- Intent management ---

// SaveIntent implements Storage.
func (s *SQLiteStorage) SaveIntent(intent domain.Intent) error {
	var dateStr *string
	if intent.Date != nil {
		d := intent.Date.UTC().Format(timeFormat)
		dateStr = &d
	}
	_, err := s.db.Exec(
		`INSERT INTO intents (id, thought_id, type, title, date, status, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		intent.ID, intent.ThoughtID, intent.Type, intent.Title, dateStr, intent.Status,
		intent.CreatedAt.UTC().Format(timeFormat),
	)
	return err
}

// GetIntent implements Storage.
func (s *SQLiteStorage) GetIntent(id string) (*domain.Intent, error) {
	row := s.db.QueryRow(
		`SELECT id, thought_id, type, title, date, status, created_at FROM intents WHERE id = ?`, id,
	)
	return s.scanOneIntent(row)
}

// GetPendingIntents implements Storage.
func (s *SQLiteStorage) GetPendingIntents() ([]domain.Intent, error) {
	rows, err := s.db.Query(
		`SELECT id, thought_id, type, title, date, status, created_at
		 FROM intents WHERE status = 'pending' ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return s.scanIntents(rows)
}

// ConfirmIntent implements Storage.
func (s *SQLiteStorage) ConfirmIntent(id string) error {
	_, err := s.db.Exec(`UPDATE intents SET status = 'confirmed' WHERE id = ?`, id)
	return err
}

// DismissIntent implements Storage.
func (s *SQLiteStorage) DismissIntent(id string) error {
	_, err := s.db.Exec(`UPDATE intents SET status = 'dismissed' WHERE id = ?`, id)
	return err
}

// --- Wellbeing / sentiment ---

// StoreSentiment implements Storage.
func (s *SQLiteStorage) StoreSentiment(thoughtID int64, score float32) error {
	_, err := s.db.Exec(
		`INSERT INTO wellbeing_signals (thought_id, score, created_at) VALUES (?, ?, ?)`,
		thoughtID, score, time.Now().UTC().Format(timeFormat),
	)
	return err
}

// GetSentimentTrend implements Storage.
func (s *SQLiteStorage) GetSentimentTrend(days int) ([]domain.WellbeingSignal, error) {
	if days <= 0 {
		days = 7
	}
	cutoff := time.Now().UTC().Add(-time.Duration(days) * 24 * time.Hour).Format(timeFormat)
	rows, err := s.db.Query(
		`SELECT thought_id, score, created_at FROM wellbeing_signals
		 WHERE created_at >= ? ORDER BY created_at ASC`,
		cutoff,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var signals []domain.WellbeingSignal
	for rows.Next() {
		var ws domain.WellbeingSignal
		var createdStr string
		if err := rows.Scan(&ws.ThoughtID, &ws.Score, &createdStr); err != nil {
			return nil, err
		}
		ws.CreatedAt, _ = time.Parse(timeFormat, createdStr)
		signals = append(signals, ws)
	}
	return signals, rows.Err()
}

// ExportData implements Storage.
func (s *SQLiteStorage) ExportData() (*ExportPayload, error) {
	rows, err := s.db.Query(
		`SELECT id, content, raw_content, window_title, app_name, url, hidden, meta, created_at, updated_at
		 FROM thoughts ORDER BY created_at ASC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	thoughts, err := s.scanThoughts(rows)
	if err != nil {
		return nil, err
	}

	intentRows, err := s.db.Query(
		`SELECT id, thought_id, type, title, date, status, created_at
		 FROM intents ORDER BY created_at ASC`,
	)
	if err != nil {
		return nil, err
	}
	defer intentRows.Close()
	intents, err := s.scanIntents(intentRows)
	if err != nil {
		return nil, err
	}

	return &ExportPayload{
		ExportedAt: time.Now().UTC(),
		Thoughts:   thoughts,
		Intents:    intents,
	}, nil
}

// ImportData implements Storage.
func (s *SQLiteStorage) ImportData(payload *ExportPayload, restore bool) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck

	if restore {
		for _, table := range []string{"wellbeing_signals", "intents", "embeddings", "tags", "thoughts"} {
			if _, err := tx.Exec(`DELETE FROM ` + table); err != nil {
				return fmt.Errorf("clear %s: %w", table, err)
			}
		}
		// Reset AUTOINCREMENT counters so future inserts start from 1.
		if _, err := tx.Exec(`DELETE FROM sqlite_sequence WHERE name IN ('thoughts','tags','wellbeing_signals')`); err != nil {
			// sqlite_sequence may not exist yet – ignore.
			_ = err
		}
	}

	// old-ID → new-ID mapping (used to fix intent.ThoughtID in additive mode).
	idMap := make(map[int64]int64, len(payload.Thoughts))

	for _, t := range payload.Thoughts {
		var metaStr *string
		if len(t.Meta) > 0 {
			b, _ := json.Marshal(t.Meta)
			s2 := string(b)
			metaStr = &s2
		}
		hidden := 0
		if t.Hidden {
			hidden = 1
		}
		createdStr := t.CreatedAt.UTC().Format(timeFormat)
		updatedStr := t.UpdatedAt.UTC().Format(timeFormat)
		if updatedStr == "" || t.UpdatedAt.IsZero() {
			updatedStr = createdStr
		}

		var res sql.Result
		if restore && t.ID != 0 {
			res, err = tx.Exec(
				`INSERT INTO thoughts (id, content, raw_content, window_title, app_name, url, hidden, meta, created_at, updated_at)
				 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				t.ID, t.Content, t.RawContent,
				t.Context.WindowTitle, t.Context.AppName, t.Context.URL,
				hidden, metaStr, createdStr, updatedStr,
			)
		} else {
			res, err = tx.Exec(
				`INSERT INTO thoughts (content, raw_content, window_title, app_name, url, hidden, meta, created_at, updated_at)
				 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				t.Content, t.RawContent,
				t.Context.WindowTitle, t.Context.AppName, t.Context.URL,
				hidden, metaStr, createdStr, updatedStr,
			)
		}
		if err != nil {
			return fmt.Errorf("import thought: %w", err)
		}
		newID, _ := res.LastInsertId()
		if restore && t.ID != 0 {
			newID = t.ID
		}
		idMap[t.ID] = newID

		for _, tag := range t.Tags {
			tagCreated := tag.CreatedAt.UTC().Format(timeFormat)
			if tag.CreatedAt.IsZero() {
				tagCreated = createdStr
			}
			if _, err := tx.Exec(
				`INSERT INTO tags (thought_id, name, source, confidence, created_at) VALUES (?, ?, ?, ?, ?)`,
				newID, tag.Name, tag.Source, tag.Confidence, tagCreated,
			); err != nil {
				return fmt.Errorf("import tag: %w", err)
			}
		}
	}

	for _, intent := range payload.Intents {
		newThoughtID, ok := idMap[intent.ThoughtID]
		if !ok {
			continue // thought was not imported; skip
		}
		var dateStr *string
		if intent.Date != nil {
			d := intent.Date.UTC().Format(timeFormat)
			dateStr = &d
		}
		intentID := intent.ID
		if !restore {
			intentID = fmt.Sprintf("imp-%d-%s", newThoughtID, intent.ID)
		}
		if _, err := tx.Exec(
			`INSERT OR IGNORE INTO intents (id, thought_id, type, title, date, status, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			intentID, newThoughtID, intent.Type, intent.Title, dateStr, intent.Status,
			intent.CreatedAt.UTC().Format(timeFormat),
		); err != nil {
			return fmt.Errorf("import intent: %w", err)
		}
	}

	return tx.Commit()
}

// GetSetting implements Storage.
func (s *SQLiteStorage) GetSetting(key string) (string, bool, error) {
	var value string
	err := s.db.QueryRow(`SELECT value FROM settings WHERE key = ?`, key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return value, true, nil
}

// SetSetting implements Storage.
func (s *SQLiteStorage) SetSetting(key, value string) error {
	_, err := s.db.Exec(
		`INSERT INTO settings (key, value) VALUES (?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		key, value,
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
	var hidden int
	var metaStr sql.NullString
	err := row.Scan(
		&t.ID, &t.Content, &t.RawContent,
		&t.Context.WindowTitle, &t.Context.AppName, &t.Context.URL,
		&hidden, &metaStr, &createdStr, &updatedStr,
	)
	if err != nil {
		return nil, err
	}
	t.Hidden = hidden != 0
	if metaStr.Valid && metaStr.String != "" {
		_ = json.Unmarshal([]byte(metaStr.String), &t.Meta)
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
		var hidden int
		var metaStr sql.NullString
		if err := rows.Scan(
			&t.ID, &t.Content, &t.RawContent,
			&t.Context.WindowTitle, &t.Context.AppName, &t.Context.URL,
			&hidden, &metaStr, &createdStr, &updatedStr,
		); err != nil {
			return nil, err
		}
		t.Hidden = hidden != 0
		if metaStr.Valid && metaStr.String != "" {
			_ = json.Unmarshal([]byte(metaStr.String), &t.Meta)
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

	args := make([]any, len(ids))
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

func (s *SQLiteStorage) scanOneIntent(row *sql.Row) (*domain.Intent, error) {
	var intent domain.Intent
	var dateStr sql.NullString
	var createdStr string
	err := row.Scan(&intent.ID, &intent.ThoughtID, &intent.Type, &intent.Title, &dateStr, &intent.Status, &createdStr)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("intent not found")
	}
	if err != nil {
		return nil, err
	}
	if dateStr.Valid && dateStr.String != "" {
		d, _ := time.Parse(timeFormat, dateStr.String)
		intent.Date = &d
	}
	intent.CreatedAt, _ = time.Parse(timeFormat, createdStr)
	return &intent, nil
}

func (s *SQLiteStorage) scanIntents(rows *sql.Rows) ([]domain.Intent, error) {
	var intents []domain.Intent
	for rows.Next() {
		var intent domain.Intent
		var dateStr sql.NullString
		var createdStr string
		if err := rows.Scan(&intent.ID, &intent.ThoughtID, &intent.Type, &intent.Title, &dateStr, &intent.Status, &createdStr); err != nil {
			return nil, err
		}
		if dateStr.Valid && dateStr.String != "" {
			d, _ := time.Parse(timeFormat, dateStr.String)
			intent.Date = &d
		}
		intent.CreatedAt, _ = time.Parse(timeFormat, createdStr)
		intents = append(intents, intent)
	}
	return intents, rows.Err()
}
