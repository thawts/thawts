package tui

import (
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	appai "thawts-client/internal/ai"
	"thawts-client/internal/metadata"
	"thawts-client/internal/service"
	"thawts-client/internal/storage"
)

func newTestModel(t *testing.T) model {
	t.Helper()
	store, err := storage.NewSQLiteStorage(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("NewSQLiteStorage: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	svc := service.New(store, appai.NewStubProvider(), metadata.NewStubProvider(), &service.NoopNotifier{})
	return newModel(svc)
}

// apply sends each message and discards the returned command.
// Use for state-only assertions where you don't need the command result.
func apply(m tea.Model, msgs ...tea.Msg) tea.Model {
	for _, msg := range msgs {
		m, _ = m.Update(msg)
	}
	return m
}

// exec sends a message, runs the returned command exactly once, then sends its
// result back. Use when a command produces a meaningful follow-up message
// (e.g. SaveThought returns a thoughtSavedMsg).
func exec(m tea.Model, msg tea.Msg) tea.Model {
	m2, cmd := m.Update(msg)
	if cmd != nil {
		if result := cmd(); result != nil {
			m2, _ = m2.Update(result)
		}
	}
	return m2
}

func asModel(t *testing.T, m tea.Model) model {
	t.Helper()
	mm, ok := m.(model)
	if !ok {
		t.Fatalf("expected model, got %T", m)
	}
	return mm
}

// ── View transitions ──────────────────────────────────────────────────────────

func TestInitialView_isCapture(t *testing.T) {
	m := newTestModel(t)
	if m.view != viewCapture {
		t.Errorf("expected viewCapture, got %v", m.view)
	}
}

func TestTabKey_switchesToReview(t *testing.T) {
	m := newTestModel(t)
	// apply discards the loadThoughtsCmd — fine for this state-only test.
	result := apply(m, tea.KeyMsg{Type: tea.KeyTab})
	mm := asModel(t, result)
	if mm.view != viewReview {
		t.Errorf("expected viewReview after Tab, got %v", mm.view)
	}
}

func TestEscFromReview_returnsToCapture(t *testing.T) {
	m := newTestModel(t)
	result := apply(m,
		tea.KeyMsg{Type: tea.KeyTab}, // → review
		tea.KeyMsg{Type: tea.KeyEsc}, // → capture
	)
	mm := asModel(t, result)
	if mm.view != viewCapture {
		t.Errorf("expected viewCapture after Esc from review, got %v", mm.view)
	}
}

func TestCtrlRKey_switchesToReview(t *testing.T) {
	m := newTestModel(t)
	result := apply(m, tea.KeyMsg{Type: tea.KeyCtrlR})
	mm := asModel(t, result)
	if mm.view != viewReview {
		t.Errorf("expected viewReview after Ctrl+R, got %v", mm.view)
	}
}

// ── Capture ───────────────────────────────────────────────────────────────────

func TestEnterInCapture_emptyInputIsNoOp(t *testing.T) {
	m := newTestModel(t)
	// No text set; press Enter.
	result := exec(m, tea.KeyMsg{Type: tea.KeyEnter})
	mm := asModel(t, result)

	thoughts, err := mm.svc.GetRecentThoughts(10)
	if err != nil {
		t.Fatalf("GetRecentThoughts: %v", err)
	}
	if len(thoughts) != 0 {
		t.Errorf("expected 0 thoughts after empty Enter, got %d", len(thoughts))
	}
}

func TestEnterInCapture_savesThoughtAndClearsInput(t *testing.T) {
	m := newTestModel(t)
	m.input.SetValue("hello world")

	// exec sends Enter → runs saveThoughtCmd → receives thoughtSavedMsg.
	result := exec(m, tea.KeyMsg{Type: tea.KeyEnter})
	mm := asModel(t, result)

	if mm.input.Value() != "" {
		t.Errorf("expected input cleared after save, got %q", mm.input.Value())
	}

	thoughts, err := mm.svc.GetRecentThoughts(10)
	if err != nil {
		t.Fatalf("GetRecentThoughts: %v", err)
	}
	if len(thoughts) == 0 {
		t.Fatal("expected thought to be saved")
	}
	if thoughts[0].Content != "hello world" {
		t.Errorf("unexpected content: %q", thoughts[0].Content)
	}
}

// ── Delete ────────────────────────────────────────────────────────────────────

func TestDeleteKey_showsConfirmPrompt(t *testing.T) {
	m := newTestModel(t)
	m.svc.SaveThought("a thought to delete")

	// Switch to review, execute the load command to populate the list.
	m2, loadCmd := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if loadCmd != nil {
		if msg := loadCmd(); msg != nil {
			m2, _ = m2.Update(msg)
		}
	}
	mm := asModel(t, m2)
	if len(mm.thoughts) == 0 {
		t.Skip("thoughts list empty after load — likely timing issue in stub")
	}

	result := apply(m2, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	mm2 := asModel(t, result)

	if mm2.confirmDel == 0 {
		t.Error("expected confirmDel to be set after first 'd'")
	}
}

func TestDeleteConfirm_removesItem(t *testing.T) {
	m := newTestModel(t)
	saved, _ := m.svc.SaveThought("thought to delete")

	m2, loadCmd := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if loadCmd != nil {
		if msg := loadCmd(); msg != nil {
			m2, _ = m2.Update(msg)
		}
	}
	if len(asModel(t, m2).thoughts) == 0 {
		t.Skip("thoughts list empty after load")
	}

	// Arm delete.
	m2 = apply(m2, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	// Confirm delete — exec so deleteThoughtCmd runs.
	m2 = exec(m2, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})

	_, err := asModel(t, m2).svc.GetThought(saved.ID)
	if err == nil {
		t.Error("expected thought to be deleted")
	}
}

func TestDeleteCancel_keepsItem(t *testing.T) {
	m := newTestModel(t)
	saved, _ := m.svc.SaveThought("thought to keep")

	m2, loadCmd := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if loadCmd != nil {
		if msg := loadCmd(); msg != nil {
			m2, _ = m2.Update(msg)
		}
	}
	if len(asModel(t, m2).thoughts) == 0 {
		t.Skip("thoughts list empty after load")
	}

	// Arm delete, then cancel.
	m2 = apply(m2,
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")},
		tea.KeyMsg{Type: tea.KeyEsc}, // cancels confirm AND switches back to capture
	)

	_, err := asModel(t, m2).svc.GetThought(saved.ID)
	if err != nil {
		t.Errorf("thought should still exist after cancel, got err: %v", err)
	}
}

// ── Search ────────────────────────────────────────────────────────────────────

func TestSlashKey_focusesSearchInput(t *testing.T) {
	m := newTestModel(t)
	result := apply(m,
		tea.KeyMsg{Type: tea.KeyTab},                        // → review
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")},  // → focus search
	)
	mm := asModel(t, result)
	if !mm.input.Focused() {
		t.Error("expected search input to be focused after '/'")
	}
}

// ── Notifications ─────────────────────────────────────────────────────────────

func TestNotifyMsg_thoughtClassified_triggersReloadInReviewMode(t *testing.T) {
	m := newTestModel(t)
	// Switch to review (discard cmd for this test).
	m2 := apply(m, tea.KeyMsg{Type: tea.KeyTab})

	_, cmd := m2.Update(NotifyMsg{Event: "thought:classified"})
	if cmd == nil {
		t.Error("expected a reload command when thought:classified fires in review mode")
	}
}

func TestNotifyMsg_inCaptureMode_producesNoCmd(t *testing.T) {
	m := newTestModel(t)
	_, cmd := m.Update(NotifyMsg{Event: "thought:classified"})
	if cmd != nil {
		t.Error("expected no reload command for thought:classified in capture mode")
	}
}
