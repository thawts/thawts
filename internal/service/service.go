// Package service contains all business logic for Thawts.
//
// Service has no dependency on any UI framework. The same struct is used by
// the Wails adapter, the TUI, and future web backends. Swap the Notifier to
// route events to the appropriate runtime.
package service

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"os/exec"
	gort "runtime"
	"strings"
	"sync"
	"time"

	"github.com/thawts/thawts/internal/ai"
	"github.com/thawts/thawts/internal/domain"
	"github.com/thawts/thawts/internal/metadata"
	"github.com/thawts/thawts/internal/storage"
)

// cosineSimilarity returns the cosine similarity in [−1, +1] between two vectors.
// Returns 0 for zero-length or dimension-mismatched vectors.
func cosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}
	denom := math.Sqrt(normA) * math.Sqrt(normB)
	if denom == 0 {
		return 0
	}
	return float32(dot / denom)
}

// Service holds all business logic. Inject dependencies via New.
type Service struct {
	store  storage.Storage
	ai     ai.Provider
	meta   metadata.Provider
	notify Notifier

	captureCtxMu sync.Mutex
	captureCtx   domain.CaptureContext
}

// New constructs a Service with its dependencies injected.
func New(store storage.Storage, aiProvider ai.Provider, metaProvider metadata.Provider, notify Notifier) *Service {
	return &Service{
		store:  store,
		ai:     aiProvider,
		meta:   metaProvider,
		notify: notify,
	}
}

// PrepareCapture collects active window metadata with a 200ms timeout and
// stores it for the next SaveThought call. Call this just before showing the
// capture UI so the context reflects the app that was active before Thawts
// gained focus.
func (s *Service) PrepareCapture() {
	ch := make(chan domain.CaptureContext, 1)
	go func() {
		ch <- domain.CaptureContext{
			WindowTitle: s.meta.GetActiveWindowTitle(),
			AppName:     s.meta.GetActiveAppName(),
			URL:         s.meta.GetActiveURL(),
		}
	}()
	var ctx domain.CaptureContext
	select {
	case ctx = <-ch:
	case <-time.After(200 * time.Millisecond):
	}
	s.captureCtxMu.Lock()
	s.captureCtx = ctx
	s.captureCtxMu.Unlock()
}

// --- Capture ---

// SaveThought persists a thought, then classifies it asynchronously.
// Returns the saved thought immediately (before classification completes).
func (s *Service) SaveThought(content string) (*domain.Thought, error) {
	s.captureCtxMu.Lock()
	ctx := s.captureCtx
	s.captureCtxMu.Unlock()

	thought, err := s.store.SaveThought(content, ctx)
	if err != nil {
		return nil, err
	}

	go s.classifyAsync(thought.ID, content)
	return thought, nil
}

func (s *Service) classifyAsync(id int64, content string) {
	ctx := context.Background()

	classification, err := s.ai.ClassifyThought(content)
	if err != nil {
		log.Printf("classifyAsync: %v", err)
		return
	}
	for _, tag := range classification.Tags {
		if err := s.store.AddTag(id, tag.Name, "ai", tag.Confidence); err != nil {
			log.Printf("addTag %q: %v", tag.Name, err)
		}
	}

	// Mishap detection — hide the thought and move it to the "Review Needed" bin.
	if s.ai.IsMishap(content, 0) {
		if err := s.store.HideThought(id); err != nil {
			log.Printf("hideThought %d: %v", id, err)
		} else {
			s.store.AddTag(id, "mishap", "ai", 0.9)
			s.notify.Emit("mishaps:changed")
		}
	} else {
		// Intent detection — only for non-mishap thoughts.
		intents, err := s.ai.DetectIntents(content)
		if err == nil && len(intents) > 0 {
			now := time.Now().UTC()
			saved := 0
			for i, intent := range intents {
				domainIntent := domain.Intent{
					ID:        fmt.Sprintf("%d-%s-%d", id, intent.Type, i),
					ThoughtID: id,
					Type:      intent.Type,
					Title:     intent.Description,
					Status:    "pending",
					CreatedAt: now,
				}
				if err := s.store.SaveIntent(domainIntent); err != nil {
					log.Printf("saveIntent: %v", err)
				} else {
					saved++
				}
			}
			if saved > 0 {
				s.notify.Emit("intents:changed")
			}
		}
	}

	// Analyze sentiment and store.
	sentimentCtx, cancelSentiment := context.WithTimeout(ctx, 2*time.Second)
	defer cancelSentiment()
	if score, err := s.ai.AnalyzeSentiment(sentimentCtx, content); err != nil {
		log.Printf("analyzeSentiment %d: %v", id, err)
	} else {
		if err := s.store.StoreSentiment(id, score); err != nil {
			log.Printf("storeSentiment %d: %v", id, err)
		} else {
			s.checkWellbeingTrend()
		}
	}

	// Embed and store the vector for future semantic search.
	embedCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if vec, err := s.ai.Embed(embedCtx, content); err != nil {
		log.Printf("embed thought %d: %v", id, err)
	} else if len(vec) > 0 {
		if err := s.store.StoreEmbedding(id, vec); err != nil {
			log.Printf("storeEmbedding %d: %v", id, err)
		}
	}

	s.notify.Emit("thought:classified", id)
}

// checkWellbeingTrend emits wellbeing:alert when the 7-day average falls below −0.4.
func (s *Service) checkWellbeingTrend() {
	avg, err := s.GetSentimentTrend(7)
	if err != nil || avg == 0 {
		return
	}
	if avg < -0.4 {
		s.notify.Emit("wellbeing:alert", avg)
	}
}

// --- Retrieval ---

// GetRecentThoughts returns the N most recently saved thoughts.
func (s *Service) GetRecentThoughts(limit int) ([]*domain.Thought, error) {
	if limit <= 0 {
		limit = 5
	}
	return s.store.GetRecentThoughts(limit)
}

// SearchThoughts returns thoughts matching the query string.
// When query is empty, returns the 20 most recent thoughts.
func (s *Service) SearchThoughts(query string) ([]*domain.Thought, error) {
	if query == "" {
		return s.store.GetRecentThoughts(20)
	}
	return s.store.SearchThoughts(query, 20)
}

// SemanticSearch returns thoughts ranked by semantic similarity to query.
// Falls back to text search when no embedding model is available.
func (s *Service) SemanticSearch(query string) ([]*domain.Thought, error) {
	if query == "" {
		return s.store.GetRecentThoughts(20)
	}

	ctx := context.Background()

	queryVec, err := s.ai.Embed(ctx, query)
	if err != nil || len(queryVec) == 0 {
		return s.store.SemanticSearch(query, 20)
	}

	candidates, err := s.store.GetRecentThoughts(200)
	if err != nil || len(candidates) == 0 {
		return s.store.SemanticSearch(query, 20)
	}

	ids := make([]int64, len(candidates))
	for i, t := range candidates {
		ids[i] = t.ID
	}

	embeddings, err := s.store.GetEmbeddings(ids)
	if err != nil || len(embeddings) == 0 {
		return s.store.SemanticSearch(query, 20)
	}

	type scored struct {
		thought *domain.Thought
		score   float32
	}
	var results []scored
	for _, t := range candidates {
		vec, ok := embeddings[t.ID]
		if !ok {
			continue
		}
		results = append(results, scored{thought: t, score: cosineSimilarity(queryVec, vec)})
	}

	for i := 1; i < len(results); i++ {
		for j := i; j > 0 && results[j].score > results[j-1].score; j-- {
			results[j], results[j-1] = results[j-1], results[j]
		}
	}

	limit := min(20, len(results))
	out := make([]*domain.Thought, limit)
	for i := range out {
		out[i] = results[i].thought
	}
	return out, nil
}

// FindRelated returns a thought semantically related to text captured more than
// 24 hours ago. Returns nil (no error) when nothing qualifies.
func (s *Service) FindRelated(text string) (*domain.Thought, error) {
	if len(text) < 3 {
		return nil, nil
	}
	candidates, err := s.SemanticSearch(text)
	if err != nil || len(candidates) == 0 {
		return nil, nil
	}
	cutoff := time.Now().Add(-24 * time.Hour)
	for _, t := range candidates {
		if t.CreatedAt.Before(cutoff) {
			return t, nil
		}
	}
	return nil, nil
}

// --- Review Mode actions ---

// UpdateThought edits the visible content of a thought.
// The original text (shadow record) is preserved.
func (s *Service) UpdateThought(id int64, content string) (*domain.Thought, error) {
	return s.store.UpdateThought(id, content)
}

// DeleteThought removes a thought permanently.
func (s *Service) DeleteThought(id int64) error {
	return s.store.DeleteThought(id)
}

// GetThought returns a single thought by ID.
func (s *Service) GetThought(id int64) (*domain.Thought, error) {
	return s.store.GetThought(id)
}

// MergeThoughts combines multiple thoughts into one, soft-deleting the originals.
func (s *Service) MergeThoughts(ids []int64) (*domain.Thought, error) {
	merged, err := s.store.MergeThoughts(ids)
	if err != nil {
		return nil, err
	}
	s.notify.Emit("thoughts:merged")
	return merged, nil
}

// CleanText returns a typo-corrected version of the thought's content for
// display purposes only. The original content is never modified.
func (s *Service) CleanText(id int64) (string, error) {
	thought, err := s.store.GetThought(id)
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return s.ai.CleanText(ctx, thought.Content)
}

// --- Mishap / "Review Needed" bin ---

// GetHiddenThoughts returns thoughts flagged as mishaps, awaiting user review.
func (s *Service) GetHiddenThoughts() ([]*domain.Thought, error) {
	return s.store.GetHiddenThoughts()
}

// UnhideThought moves a thought out of the mishap bin.
func (s *Service) UnhideThought(id int64) error {
	return s.store.UnhideThought(id)
}

// --- Intent Actions ---

// GetPendingIntents returns all intents awaiting user confirmation.
func (s *Service) GetPendingIntents() ([]domain.Intent, error) {
	return s.store.GetPendingIntents()
}

// ConfirmIntent marks an intent as confirmed and attempts to create a native
// calendar or reminder entry (macOS only, best-effort).
func (s *Service) ConfirmIntent(intentID string) error {
	if err := s.store.ConfirmIntent(intentID); err != nil {
		return err
	}
	intent, err := s.store.GetIntent(intentID)
	if err == nil {
		go s.createNativeEvent(intent)
	}
	s.notify.Emit("intents:changed")
	return nil
}

// DismissIntent marks an intent as dismissed.
func (s *Service) DismissIntent(intentID string) error {
	if err := s.store.DismissIntent(intentID); err != nil {
		return err
	}
	s.notify.Emit("intents:changed")
	return nil
}

// createNativeEvent creates a native calendar or reminder entry via AppleScript
// on macOS. Best-effort: errors are logged but not surfaced.
func (s *Service) createNativeEvent(intent *domain.Intent) {
	if gort.GOOS != "darwin" {
		return
	}
	title := strings.ReplaceAll(intent.Title, `"`, `'`)

	var script string
	switch intent.Type {
	case "calendar":
		script = fmt.Sprintf(`tell application "Calendar"
			tell calendar "Calendar"
				make new event at end of events with properties {summary:"%s"}
			end tell
		end tell`, title)
	case "task", "reminder":
		script = fmt.Sprintf(`tell application "Reminders"
			tell list "Reminders"
				make new reminder with properties {name:"%s"}
			end tell
		end tell`, title)
	default:
		return
	}

	if err := exec.Command("osascript", "-e", script).Run(); err != nil {
		log.Printf("createNativeEvent %q: %v", intent.Title, err)
	}
}

// --- Wellbeing ---

// --- Import / Export ---

// ExportToJSON writes a full JSON snapshot of all thoughts and intents to path.
// Returns the number of thoughts exported.
func (s *Service) ExportToJSON(path string) (int, error) {
	payload, err := s.store.ExportData()
	if err != nil {
		return 0, fmt.Errorf("export: %w", err)
	}
	f, err := os.Create(path)
	if err != nil {
		return 0, fmt.Errorf("create file: %w", err)
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(payload); err != nil {
		return 0, fmt.Errorf("encode json: %w", err)
	}
	return len(payload.Thoughts), nil
}

// ExportToCSV writes thoughts as CSV to path. Tags are pipe-separated in one column.
// Returns the number of thoughts exported.
func (s *Service) ExportToCSV(path string) (int, error) {
	payload, err := s.store.ExportData()
	if err != nil {
		return 0, fmt.Errorf("export: %w", err)
	}
	f, err := os.Create(path)
	if err != nil {
		return 0, fmt.Errorf("create file: %w", err)
	}
	defer f.Close()
	w := csv.NewWriter(f)
	defer w.Flush()
	if err := w.Write([]string{"id", "content", "raw_content", "created_at", "updated_at", "hidden", "tags", "window_title", "app_name", "url"}); err != nil {
		return 0, err
	}
	for _, t := range payload.Thoughts {
		tagNames := make([]string, len(t.Tags))
		for i, tag := range t.Tags {
			tagNames[i] = tag.Name
		}
		hidden := "0"
		if t.Hidden {
			hidden = "1"
		}
		if err := w.Write([]string{
			fmt.Sprintf("%d", t.ID),
			t.Content,
			t.RawContent,
			t.CreatedAt.UTC().Format(time.RFC3339),
			t.UpdatedAt.UTC().Format(time.RFC3339),
			hidden,
			strings.Join(tagNames, "|"),
			t.Context.WindowTitle,
			t.Context.AppName,
			t.Context.URL,
		}); err != nil {
			return 0, err
		}
	}
	return len(payload.Thoughts), nil
}

// ImportFromJSON reads a JSON snapshot from path and imports it.
// When restore is true all existing data is deleted first.
// Returns the number of thoughts imported.
func (s *Service) ImportFromJSON(path string, restore bool) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, fmt.Errorf("open file: %w", err)
	}
	defer f.Close()
	var payload storage.ExportPayload
	if err := json.NewDecoder(f).Decode(&payload); err != nil {
		return 0, fmt.Errorf("decode json: %w", err)
	}
	if err := s.store.ImportData(&payload, restore); err != nil {
		return 0, fmt.Errorf("import: %w", err)
	}
	return len(payload.Thoughts), nil
}

// ImportFromCSV reads a CSV file from path and imports the thoughts.
// Expected header columns (case-insensitive): content, raw_content, created_at,
// updated_at, hidden, tags (pipe-separated), window_title, app_name, url.
// Only "content" is required. When restore is true existing data is deleted first.
// Returns the number of thoughts imported.
func (s *Service) ImportFromCSV(path string, restore bool) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.TrimLeadingSpace = true
	headers, err := r.Read()
	if err != nil {
		return 0, fmt.Errorf("read csv header: %w", err)
	}
	colIdx := make(map[string]int, len(headers))
	for i, h := range headers {
		colIdx[strings.ToLower(strings.TrimSpace(h))] = i
	}
	if _, ok := colIdx["content"]; !ok {
		return 0, fmt.Errorf("CSV missing required column 'content'")
	}

	col := func(row []string, name string) string {
		i, ok := colIdx[name]
		if !ok || i >= len(row) {
			return ""
		}
		return strings.TrimSpace(row[i])
	}

	now := time.Now().UTC()
	var thoughts []*domain.Thought
	for {
		row, err := r.Read()
		if err != nil {
			break
		}
		content := col(row, "content")
		if content == "" {
			continue
		}
		rawContent := col(row, "raw_content")
		if rawContent == "" {
			rawContent = content
		}
		createdAt := now
		if v := col(row, "created_at"); v != "" {
			if t, err := time.Parse(time.RFC3339, v); err == nil {
				createdAt = t.UTC()
			}
		}
		updatedAt := createdAt
		if v := col(row, "updated_at"); v != "" {
			if t, err := time.Parse(time.RFC3339, v); err == nil {
				updatedAt = t.UTC()
			}
		}
		hidden := col(row, "hidden") == "1" || strings.EqualFold(col(row, "hidden"), "true")
		var tags []domain.Tag
		if tagStr := col(row, "tags"); tagStr != "" {
			for _, name := range strings.Split(tagStr, "|") {
				name = strings.TrimSpace(name)
				if name != "" {
					tags = append(tags, domain.Tag{Name: name, Source: "import", Confidence: 1.0, CreatedAt: createdAt})
				}
			}
		}
		thoughts = append(thoughts, &domain.Thought{
			Content:    content,
			RawContent: rawContent,
			Context: domain.CaptureContext{
				WindowTitle: col(row, "window_title"),
				AppName:     col(row, "app_name"),
				URL:         col(row, "url"),
			},
			Tags:      tags,
			Hidden:    hidden,
			CreatedAt: createdAt,
			UpdatedAt: updatedAt,
		})
	}

	payload := &storage.ExportPayload{Thoughts: thoughts}
	if err := s.store.ImportData(payload, restore); err != nil {
		return 0, fmt.Errorf("import: %w", err)
	}
	return len(thoughts), nil
}

// --- Settings ---

// GetSettings returns the current user settings, filling in defaults for any unset keys.
func (s *Service) GetSettings() (domain.Settings, error) {
	settings := domain.Settings{
		CaptureHotkey: defaultCaptureHotkey,
		ReviewHotkey:  defaultReviewHotkey,
	}
	if v, ok, err := s.store.GetSetting("capture_hotkey"); err != nil {
		return settings, err
	} else if ok {
		settings.CaptureHotkey = v
	}
	if v, ok, err := s.store.GetSetting("review_hotkey"); err != nil {
		return settings, err
	} else if ok {
		settings.ReviewHotkey = v
	}
	if v, ok, err := s.store.GetSetting("launch_at_login"); err != nil {
		return settings, err
	} else if ok {
		settings.LaunchAtLogin = v == "true"
	}
	return settings, nil
}

// SaveSettings persists all settings fields.
func (s *Service) SaveSettings(settings domain.Settings) error {
	if err := s.store.SetSetting("capture_hotkey", settings.CaptureHotkey); err != nil {
		return err
	}
	if err := s.store.SetSetting("review_hotkey", settings.ReviewHotkey); err != nil {
		return err
	}
	val := "false"
	if settings.LaunchAtLogin {
		val = "true"
	}
	return s.store.SetSetting("launch_at_login", val)
}

// --- Wellbeing ---

// GetSentimentTrend returns the rolling average sentiment score over the last
// `days` days. Returns 0 when no signals are recorded.
func (s *Service) GetSentimentTrend(days int) (float32, error) {
	if days <= 0 {
		days = 7
	}
	signals, err := s.store.GetSentimentTrend(days)
	if err != nil || len(signals) == 0 {
		return 0, err
	}
	var sum float32
	for _, sig := range signals {
		sum += sig.Score
	}
	return sum / float32(len(signals)), nil
}
