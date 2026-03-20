package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/thawts/thawts/internal/domain"
	"github.com/thawts/thawts/internal/service"
)

// ── View kinds ────────────────────────────────────────────────────────────────

type viewKind int

const (
	viewCapture viewKind = iota
	viewReview
)

// ── Messages ──────────────────────────────────────────────────────────────────

// NotifyMsg carries async events emitted by the service (e.g. thought:classified).
type NotifyMsg struct {
	Event string
	Data  []any
}

type thoughtSavedMsg struct{ thought *domain.Thought }
type thoughtsLoadedMsg struct{ thoughts []*domain.Thought }
type thoughtDeletedMsg struct{ id int64 }
type errMsg struct{ err error }

// ── List item ─────────────────────────────────────────────────────────────────

type thoughtItem struct{ t *domain.Thought }

func (i thoughtItem) Title() string {
	content := i.t.Content
	if len(content) > 80 {
		content = content[:80] + "…"
	}
	return content
}

func (i thoughtItem) Description() string {
	parts := make([]string, 0, len(i.t.Tags)+1)
	for _, tag := range i.t.Tags {
		parts = append(parts, tag.Name)
	}
	parts = append(parts, formatAge(i.t.CreatedAt))
	return strings.Join(parts, "  ·  ")
}

func (i thoughtItem) FilterValue() string { return i.t.Content }

func formatAge(t time.Time) string {
	d := time.Since(t)
	if d < time.Minute {
		return "just now"
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	}
	return fmt.Sprintf("%dd ago", int(d.Hours()/24))
}

// ── Styles ────────────────────────────────────────────────────────────────────

var (
	stylePrimary   = lipgloss.NewStyle().Foreground(lipgloss.Color("#7B68EE"))
	styleDim       = lipgloss.NewStyle().Foreground(lipgloss.Color("#555"))
	styleStatus    = lipgloss.NewStyle().Foreground(lipgloss.Color("#888"))
	styleConfirm   = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF6464")).Bold(true)
	styleCapture   = lipgloss.NewStyle().Padding(1, 2)
	styleInputPfx  = lipgloss.NewStyle().Foreground(lipgloss.Color("#7B68EE")).Bold(true)
	styleHelp      = lipgloss.NewStyle().Foreground(lipgloss.Color("#444"))
)

const captureHelp = "enter: save  tab: review  esc: quit"
const reviewHelp  = "↑/↓: navigate  /: search  d: delete  e: edit  esc: capture"

// ── Model ─────────────────────────────────────────────────────────────────────

type model struct {
	svc        *service.Service
	view       viewKind
	input      textinput.Model
	list       list.Model
	thoughts   []*domain.Thought
	confirmDel int64  // ID of thought awaiting delete confirmation; 0 = none
	editing    bool   // true when input is used as an inline editor
	editID     int64  // ID of thought being edited
	status     string // transient status message
	width      int
	height     int
}

func newModel(svc *service.Service) model {
	ti := textinput.New()
	ti.Placeholder = "What's on your mind…"
	ti.Focus()
	ti.CharLimit = 500

	delegate := list.NewDefaultDelegate()
	l := list.New(nil, delegate, 0, 0)
	l.Title = "Thoughts"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false) // we handle search ourselves via the input

	return model{
		svc:   svc,
		view:  viewCapture,
		input: ti,
		list:  l,
	}
}

// ── Init ──────────────────────────────────────────────────────────────────────

func (m model) Init() tea.Cmd {
	return textinput.Blink
}

// ── Update ────────────────────────────────────────────────────────────────────

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.list.SetSize(msg.Width, msg.Height-4)
		return m, nil

	// ── Async results ──────────────────────────────────────────────────────
	case thoughtSavedMsg:
		m.input.SetValue("")
		m.status = "✓ saved"
		if m.view == viewReview {
			return m, loadThoughtsCmd(m.svc, "")
		}
		return m, nil

	case thoughtsLoadedMsg:
		m.thoughts = msg.thoughts
		items := make([]list.Item, len(msg.thoughts))
		for i, t := range msg.thoughts {
			items[i] = thoughtItem{t}
		}
		m.list.SetItems(items)
		return m, nil

	case thoughtDeletedMsg:
		m.confirmDel = 0
		m.status = "deleted"
		return m, loadThoughtsCmd(m.svc, m.input.Value())

	case errMsg:
		m.status = "error: " + msg.err.Error()
		return m, nil

	// ── Service notifications (async classification etc.) ──────────────────
	case NotifyMsg:
		if msg.Event == "thought:classified" && m.view == viewReview {
			return m, loadThoughtsCmd(m.svc, m.input.Value())
		}
		return m, nil

	// ── Keyboard ──────────────────────────────────────────────────────────
	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	// Delegate to bubbles components.
	var cmd tea.Cmd
	if m.view == viewCapture || m.editing {
		m.input, cmd = m.input.Update(msg)
		// Live search while in review mode.
		if m.view == viewReview && !m.editing {
			return m, tea.Batch(cmd, loadThoughtsCmd(m.svc, m.input.Value()))
		}
		return m, cmd
	}
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Cancel pending delete on any key other than 'd'.
	if m.confirmDel != 0 && msg.String() != "d" {
		m.confirmDel = 0
		m.status = ""
	}

	switch msg.Type {
	case tea.KeyTab:
		if m.view == viewCapture {
			return m.switchToReview()
		}
		return m.switchToCapture()

	case tea.KeyEsc:
		if m.editing {
			m.editing = false
			m.editID = 0
			m.input.SetValue("")
			m.input.Blur()
			return m, nil
		}
		if m.view == viewReview {
			return m.switchToCapture()
		}
		return m, tea.Quit

	case tea.KeyEnter:
		if m.editing {
			return m.submitEdit()
		}
		if m.view == viewCapture {
			return m.submitCapture()
		}

	case tea.KeyCtrlR:
		if m.view == viewCapture {
			return m.switchToReview()
		}

	default:
		if m.view == viewReview && !m.input.Focused() {
			switch msg.String() {
			case "d":
				return m.handleDelete()
			case "e":
				return m.handleEdit()
			case "/":
				m.input.Focus()
				m.input.Placeholder = "Search…"
				return m, textinput.Blink
			}
		}
	}

	// Delegate to active component.
	var cmd tea.Cmd
	if m.view == viewCapture || m.input.Focused() {
		m.input, cmd = m.input.Update(msg)
		if m.view == viewReview {
			return m, tea.Batch(cmd, loadThoughtsCmd(m.svc, m.input.Value()))
		}
		return m, cmd
	}
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

// ── Actions ───────────────────────────────────────────────────────────────────

func (m model) switchToReview() (model, tea.Cmd) {
	m.view = viewReview
	m.input.Placeholder = "Search…"
	m.input.SetValue("")
	m.input.Blur()
	m.status = ""
	return m, loadThoughtsCmd(m.svc, "")
}

func (m model) switchToCapture() (model, tea.Cmd) {
	m.view = viewCapture
	m.input.Placeholder = "What's on your mind…"
	m.input.SetValue("")
	m.input.Focus()
	m.confirmDel = 0
	m.editing = false
	m.status = ""
	return m, textinput.Blink
}

func (m model) submitCapture() (model, tea.Cmd) {
	text := strings.TrimSpace(m.input.Value())
	if text == "" {
		return m, nil
	}
	return m, saveThoughtCmd(m.svc, text)
}

func (m model) submitEdit() (model, tea.Cmd) {
	text := strings.TrimSpace(m.input.Value())
	id := m.editID
	m.editing = false
	m.editID = 0
	m.input.SetValue("")
	m.input.Blur()
	if text == "" || id == 0 {
		return m, nil
	}
	return m, func() tea.Msg {
		if _, err := m.svc.UpdateThought(id, text); err != nil {
			return errMsg{err}
		}
		return thoughtDeletedMsg{id: 0} // reuse to trigger reload
	}
}

func (m model) handleDelete() (model, tea.Cmd) {
	item, ok := m.list.SelectedItem().(thoughtItem)
	if !ok {
		return m, nil
	}
	id := item.t.ID
	if m.confirmDel == id {
		// Second 'd' — execute deletion.
		m.confirmDel = 0
		return m, deleteThoughtCmd(m.svc, id)
	}
	// First 'd' — arm confirmation.
	m.confirmDel = id
	m.status = "press d again to delete, any other key cancels"
	return m, nil
}

func (m model) handleEdit() (model, tea.Cmd) {
	item, ok := m.list.SelectedItem().(thoughtItem)
	if !ok {
		return m, nil
	}
	m.editing = true
	m.editID = item.t.ID
	m.input.SetValue(item.t.Content)
	m.input.Focus()
	return m, textinput.Blink
}

// ── Commands ──────────────────────────────────────────────────────────────────

func saveThoughtCmd(svc *service.Service, text string) tea.Cmd {
	return func() tea.Msg {
		t, err := svc.SaveThought(text)
		if err != nil {
			return errMsg{err}
		}
		return thoughtSavedMsg{t}
	}
}

func loadThoughtsCmd(svc *service.Service, query string) tea.Cmd {
	return func() tea.Msg {
		var thoughts []*domain.Thought
		var err error
		if query == "" {
			thoughts, err = svc.GetRecentThoughts(50)
		} else {
			thoughts, err = svc.SearchThoughts(query)
		}
		if err != nil {
			return errMsg{err}
		}
		return thoughtsLoadedMsg{thoughts}
	}
}

func deleteThoughtCmd(svc *service.Service, id int64) tea.Cmd {
	return func() tea.Msg {
		if err := svc.DeleteThought(id); err != nil {
			return errMsg{err}
		}
		return thoughtDeletedMsg{id}
	}
}

// ── View ──────────────────────────────────────────────────────────────────────

func (m model) View() string {
	switch m.view {
	case viewCapture:
		return m.captureView()
	case viewReview:
		return m.reviewView()
	}
	return ""
}

func (m model) captureView() string {
	prompt := styleInputPfx.Render("› ")
	input := m.input.View()

	status := ""
	if m.status != "" {
		status = "\n" + styleStatus.Render(m.status)
	}

	help := styleHelp.Render(captureHelp)

	return styleCapture.Render(
		prompt+input+status+"\n\n"+help,
	)
}

func (m model) reviewView() string {
	header := stylePrimary.Render("Thoughts")
	if m.input.Focused() {
		header = stylePrimary.Render("Search: ") + m.input.View()
	} else if m.editing {
		header = stylePrimary.Render("Edit: ") + m.input.View()
	}

	status := ""
	if m.confirmDel != 0 {
		status = "\n" + styleConfirm.Render(m.status)
	} else if m.status != "" {
		status = "\n" + styleStatus.Render(m.status)
	}

	body := m.list.View()
	help := styleHelp.Render(reviewHelp)
	count := styleDim.Render(fmt.Sprintf("%d thoughts", len(m.thoughts)))

	return header + status + "\n" + body + "\n" + count + "  " + help
}
