import './style.css';
import {
  SaveThought,
  SearchThoughts,
  UpdateThought,
  DeleteThought,
  GetThought,
  ShowCapture,
  ShowReview,
  HideWindow,
  SetCaptureHeight,
} from '../wailsjs/go/app/App.js';
import { EventsOn } from '../wailsjs/runtime/runtime.js';

// ─── State ────────────────────────────────────────────────────────────────────

let mode = 'braindump'; // 'braindump' | 'review'
let reviewThoughts = [];
let selectedThoughtId = null;
let reviewFilter = 'all';
let searchTimer = null;

// ─── Bootstrap ───────────────────────────────────────────────────────────────

document.addEventListener('DOMContentLoaded', () => {
  buildApp();
  window.addEventListener('blur', () => {
    if (mode === 'braindump') setTimeout(() => HideWindow(), 200);
  });
});

EventsOn('mode:capture', () => enterBraindump());
EventsOn('mode:review',  () => enterReview());
EventsOn('thought:classified', () => { if (mode === 'review') refreshGarden(); });

function buildApp() {
  document.getElementById('app').innerHTML = `
    <div id="shell">
      <div id="input-row">
        <button id="back-btn" title="Back to capture" aria-label="Back to capture">&#8592;</button>
        <input
          id="thought-input"
          type="text"
          placeholder="What's on your mind…"
          autocomplete="off"
          spellcheck="false"
        />
      </div>
      <div id="garden-area" style="display:none"></div>
    </div>
  `;

  const input = document.getElementById('thought-input');
  input.addEventListener('keydown', onInputKeydown);
  input.addEventListener('input', onInputChange);

  document.getElementById('back-btn').addEventListener('click', () => ShowCapture());

  input.focus();
}

// ─── Mode transitions ─────────────────────────────────────────────────────────

function enterBraindump() {
  mode = 'braindump';
  document.getElementById('shell').dataset.mode = 'braindump';
  document.getElementById('garden-area').style.display = 'none';
  document.getElementById('garden-area').innerHTML = '';
  const input = document.getElementById('thought-input');
  if (input) { input.placeholder = 'What\'s on your mind…'; input.value = ''; input.focus(); }
  SetCaptureHeight(60);
}

function enterReview() {
  mode = 'review';
  document.getElementById('shell').dataset.mode = 'review';
  const gardenArea = document.getElementById('garden-area');
  gardenArea.style.display = 'flex';
  mountGarden(gardenArea);
  const input = document.getElementById('thought-input');
  if (input) { input.placeholder = 'Search garden…'; input.value = ''; input.focus(); }
  refreshGarden();
}

// ─── Input ────────────────────────────────────────────────────────────────────

async function onInputKeydown(e) {
  const input = e.target;
  const text = input.value.trim();

  if (e.key === 'Escape') {
    e.preventDefault();
    if (mode === 'review' && !text) {
      ShowCapture();
    } else {
      HideWindow();
    }
    return;
  }

  if (e.key === 'Enter') {
    e.preventDefault();

    // Slash commands (both modes)
    if (text === '/review') {
      input.value = '';
      ShowReview(); // Go resizes window and emits mode:review
      return;
    }

    if (mode === 'braindump' || mode === 'review') {
      if (!text || text.startsWith('/')) return;
      input.value = '';
      input.disabled = true;
      try {
        await SaveThought(text);
        if (mode === 'review') await refreshGarden();
      } catch (err) {
        console.error('SaveThought failed:', err);
        input.value = text;
      } finally {
        input.disabled = false;
        input.focus();
      }
    }
  }
}

function onInputChange() {
  if (mode === 'review') {
    clearTimeout(searchTimer);
    searchTimer = setTimeout(refreshGarden, 250);
  }
}

// ─── Garden (review mode) ─────────────────────────────────────────────────────

function mountGarden(container) {
  selectedThoughtId = null;
  reviewFilter = 'all';

  container.innerHTML = `
    <div id="review-layout">
      <aside id="sidebar">
        <nav id="sidebar-nav">
          <button class="nav-btn active" data-filter="all">All</button>
          <button class="nav-btn" data-filter="todo">To-do</button>
          <button class="nav-btn" data-filter="idea">Ideas</button>
          <button class="nav-btn" data-filter="calendar">Calendar</button>
          <button class="nav-btn" data-filter="reminder">Reminders</button>
        </nav>
      </aside>
      <main id="stream"></main>
      <aside id="inspector"></aside>
    </div>
  `;

  document.querySelectorAll('.nav-btn').forEach(btn => {
    btn.addEventListener('click', () => {
      document.querySelectorAll('.nav-btn').forEach(b => b.classList.remove('active'));
      btn.classList.add('active');
      reviewFilter = btn.dataset.filter;
      refreshGarden();
    });
  });
}

async function refreshGarden() {
  const query = document.getElementById('thought-input')?.value.trim() || '';
  try {
    const all = await SearchThoughts(query);
    reviewThoughts = (all || []).filter(t => {
      if (reviewFilter === 'all') return true;
      return (t.tags || []).some(tag => tag.name === reviewFilter);
    });
  } catch (e) {
    console.error('refreshGarden failed:', e);
    reviewThoughts = [];
  }
  renderStream();
}

function renderStream() {
  const stream = document.getElementById('stream');
  if (!stream) return;

  if (reviewThoughts.length === 0) {
    stream.innerHTML = `<div class="empty-state">No thoughts yet.<br>Start capturing with <kbd>Ctrl+Shift+Space</kbd>.</div>`;
    return;
  }

  const query = document.getElementById('thought-input')?.value.trim() || '';
  stream.innerHTML = reviewThoughts.map(t => {
    const content = query ? highlight(t.content, query) : escapeHtml(t.content);
    const isSelected = t.id === selectedThoughtId;
    return `
      <div class="stream-item${isSelected ? ' selected' : ''}" data-id="${t.id}">
        <div class="stream-content">${content}</div>
        <div class="stream-footer">
          ${renderTags(t.tags || [])}
          <span class="stream-time">${formatRelativeTime(t.created_at)}</span>
        </div>
      </div>
    `;
  }).join('');

  stream.querySelectorAll('.stream-item').forEach(el => {
    el.addEventListener('click', () => selectThought(Number(el.dataset.id)));
  });
}

async function selectThought(id) {
  selectedThoughtId = id;
  renderStream();
  try {
    const t = await GetThought(id);
    renderInspector(t);
  } catch (_) {
    renderInspector(null);
  }
}

function renderInspector(t) {
  const inspector = document.getElementById('inspector');
  if (!inspector) return;
  if (!t) { inspector.innerHTML = ''; return; }

  const shadowSection = t.content !== t.raw_content ? `
    <div class="inspector-section shadow-section">
      <label class="inspector-label">Original (shadow record)</label>
      <div class="shadow-text">${escapeHtml(t.raw_content)}</div>
    </div>
  ` : '';

  const contextSection = t.context && (t.context.app_name || t.context.window_title) ? `
    <div class="inspector-section">
      <label class="inspector-label">Context</label>
      <div class="context-info">
        ${t.context.app_name ? `<span class="ctx-app">${escapeHtml(t.context.app_name)}</span>` : ''}
        ${t.context.window_title ? `<span class="ctx-window">${escapeHtml(t.context.window_title)}</span>` : ''}
      </div>
    </div>
  ` : '';

  inspector.innerHTML = `
    <div id="inspector-inner">
      <div class="inspector-section">
        <label class="inspector-label">Content</label>
        <textarea id="inspector-content" rows="6">${escapeHtml(t.content)}</textarea>
      </div>
      ${shadowSection}
      <div class="inspector-section">
        <label class="inspector-label">Tags</label>
        <div>${renderTags(t.tags || [])}</div>
      </div>
      ${contextSection}
      <div class="inspector-actions">
        <button id="btn-save-edit" class="btn-primary">Save</button>
        <button id="btn-delete" class="btn-danger">Delete</button>
      </div>
      <div class="inspector-times">
        <span>Created ${formatAbsoluteTime(t.created_at)}</span>
      </div>
    </div>
  `;

  document.getElementById('btn-save-edit').addEventListener('click', async () => {
    const newContent = document.getElementById('inspector-content').value.trim();
    if (!newContent || newContent === t.content) return;
    try {
      const updated = await UpdateThought(t.id, newContent);
      const idx = reviewThoughts.findIndex(x => x.id === t.id);
      if (idx !== -1) reviewThoughts[idx] = updated;
      renderStream();
      renderInspector(updated);
    } catch (e) { console.error(e); }
  });

  document.getElementById('btn-delete').addEventListener('click', async () => {
    if (!confirm('Delete this thought? This cannot be undone.')) return;
    try {
      await DeleteThought(t.id);
      selectedThoughtId = null;
      inspector.innerHTML = '';
      reviewThoughts = reviewThoughts.filter(x => x.id !== t.id);
      renderStream();
    } catch (e) { console.error(e); }
  });
}

// ─── Shared helpers ───────────────────────────────────────────────────────────

const TAG_COLORS = {
  todo:      '#ff6464',
  idea:      '#64c8ff',
  calendar:  '#ffc864',
  reminder:  '#ff64ff',
  question:  '#aaffaa',
  quote:     '#aaaaaa',
  finance:   '#ffd700',
};

function renderTags(tags) {
  if (!tags || tags.length === 0) return '';
  return tags.map(tag => {
    const color = TAG_COLORS[tag.name] || '#888';
    const border = tag.source === 'ai' ? 'dashed' : 'solid';
    return `<span class="tag" style="border:1px ${border} ${color};color:${color}">${escapeHtml(tag.name)}</span>`;
  }).join('');
}

function escapeHtml(str) {
  if (!str) return '';
  return str
    .replace(/&/g, '&amp;').replace(/</g, '&lt;')
    .replace(/>/g, '&gt;').replace(/"/g, '&quot;');
}

function highlight(text, query) {
  const safe = escapeHtml(text);
  const re = new RegExp(escapeHtml(query).replace(/[.*+?^${}()|[\]\\]/g, '\\$&'), 'gi');
  return safe.replace(re, m => `<mark>${m}</mark>`);
}

function formatRelativeTime(isoStr) {
  if (!isoStr) return '';
  const diff = Date.now() - new Date(isoStr).getTime();
  const s = Math.floor(diff / 1000);
  if (s < 60) return 'just now';
  const m = Math.floor(s / 60);
  if (m < 60) return `${m}m ago`;
  const h = Math.floor(m / 60);
  if (h < 24) return `${h}h ago`;
  return `${Math.floor(h / 24)}d ago`;
}

function formatAbsoluteTime(isoStr) {
  if (!isoStr) return '';
  return new Date(isoStr).toLocaleString(undefined, {
    month: 'short', day: 'numeric',
    hour: '2-digit', minute: '2-digit',
  });
}
