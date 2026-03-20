import './style.css';
import { setupEscHandler } from './esc-handler.js';
import {
  SaveThought,
  SearchThoughts,
  UpdateThought,
  DeleteThought,
  GetThought,
  GetHiddenThoughts,
  UnhideThought,
  ShowCapture,
  ShowReview,
  HideWindow,
  SetCaptureHeight,
  FindRelated,
  GetPendingIntents,
  ConfirmIntent,
  DismissIntent,
  GetSentimentTrend,
  MergeThoughts,
  CleanText,
} from '../bindings/github.com/thawts/thawts/internal/app/app.js';
import { Events } from '@wailsio/runtime';

// ─── Slash commands ───────────────────────────────────────────────────────────

const SLASH_COMMANDS = [
  { cmd: '/review', description: 'Open the garden' },
];

// ─── State ────────────────────────────────────────────────────────────────────

let mode = 'braindump'; // 'braindump' | 'review'
let reviewThoughts = [];
let hiddenThoughts = [];
let pendingIntents = [];
let selectedThoughtId = null;
let reviewFilter = 'all'; // 'all' | tag name | 'mishap' | 'actions'
let searchTimer = null;
let relatedTimer = null;  // debounce for proactive synthesis (DELTA-3c)
let slashIndex = -1;     // highlighted suggestion index, -1 = none
let slashVisible = [];   // currently visible filtered commands
let mergeSelectionIds = new Set(); // IDs selected for merge (DELTA-7a)

// ─── Bootstrap ───────────────────────────────────────────────────────────────

document.addEventListener('DOMContentLoaded', () => {
  buildApp();
  setupEscHandler(() => mode, HideWindow, ShowCapture);
  window.addEventListener('blur', () => {
    if (mode === 'braindump') setTimeout(() => HideWindow(), 200);
  });
});

Events.On('mode:capture', () => enterBraindump());
Events.On('mode:review',  () => enterReview());
Events.On('thought:classified', () => { if (mode === 'review') refreshGarden(); });
Events.On('mishaps:changed', () => { if (mode === 'review') refreshMishapBadge(); });
Events.On('intents:changed', () => { if (mode === 'review') refreshIntentsBadge(); });
Events.On('thoughts:merged', () => { if (mode === 'review') refreshGarden(); });
Events.On('wellbeing:alert', () => { if (mode === 'review') checkAndShowWellbeingCard(); });

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
      <div id="slash-suggestions" style="display:none"></div>
      <div id="related-hint" style="display:none"></div>
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
  dismissSlash();
  dismissRelatedHint();
  document.getElementById('shell').dataset.mode = 'braindump';
  document.getElementById('garden-area').style.display = 'none';
  document.getElementById('garden-area').innerHTML = '';
  const input = document.getElementById('thought-input');
  if (input) { input.placeholder = 'What\'s on your mind…'; input.value = ''; setTimeout(() => input.focus(), 50); }
  SetCaptureHeight(60);
}

function enterReview() {
  mode = 'review';
  document.getElementById('shell').dataset.mode = 'review';
  const gardenArea = document.getElementById('garden-area');
  gardenArea.style.display = 'flex';
  mountGarden(gardenArea);
  const input = document.getElementById('thought-input');
  if (input) { input.placeholder = 'Search garden…'; input.value = ''; setTimeout(() => input.focus(), 50); }
  refreshGarden();
  checkAndShowWellbeingCard();
}

// ─── Input ────────────────────────────────────────────────────────────────────

async function onInputKeydown(e) {
  const input = e.target;
  const text = input.value.trim();

  // Arrow navigation and Enter/Escape/Tab when suggestions are open
  if (slashVisible.length > 0) {
    if (e.key === 'ArrowDown') {
      e.preventDefault();
      slashIndex = (slashIndex + 1) % slashVisible.length;
      renderSlashSuggestions();
      return;
    }
    if (e.key === 'ArrowUp') {
      e.preventDefault();
      slashIndex = slashIndex <= 0 ? slashVisible.length - 1 : slashIndex - 1;
      renderSlashSuggestions();
      return;
    }
    if (e.key === 'Enter' || e.key === 'Tab') {
      e.preventDefault();
      const target = slashIndex >= 0 ? slashVisible[slashIndex] : slashVisible[0];
      if (target) applySlashCommand(target.cmd);
      return;
    }
    if (e.key === 'Escape') {
      e.preventDefault();
      dismissSlash();
      return;
    }
  }

  // Arrow navigation through the thought stream in review mode (only when input is empty)
  if (mode === 'review' && !text && slashVisible.length === 0 && reviewFilter !== 'mishap' && reviewFilter !== 'actions') {
    if (e.key === 'ArrowDown') {
      e.preventDefault();
      navigateThoughts(1);
      return;
    }
    if (e.key === 'ArrowUp') {
      e.preventDefault();
      navigateThoughts(-1);
      return;
    }
  }

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
      dismissRelatedHint();
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
  const val = document.getElementById('thought-input')?.value ?? '';

  if (val.startsWith('/')) {
    updateSlashSuggestions(val);
  } else {
    dismissSlash();
  }

  if (mode === 'review' && !val.startsWith('/')) {
    clearTimeout(searchTimer);
    searchTimer = setTimeout(refreshGarden, 250);
  }

  // DELTA-3c: Proactive synthesis — show a related memory hint after 1.5s of pause.
  if (mode === 'braindump' && val.trim().length >= 3 && !val.startsWith('/')) {
    clearTimeout(relatedTimer);
    relatedTimer = setTimeout(() => checkRelated(val.trim()), 1500);
  } else {
    clearTimeout(relatedTimer);
    dismissRelatedHint();
  }
}

async function checkRelated(text) {
  try {
    const thought = await FindRelated(text);
    if (thought && thought.id) {
      showRelatedHint(thought);
    } else {
      dismissRelatedHint();
    }
  } catch (_) {
    dismissRelatedHint();
  }
}

function showRelatedHint(thought) {
  const el = document.getElementById('related-hint');
  if (!el) return;
  const preview = thought.content.length > 60
    ? thought.content.slice(0, 60) + '…'
    : thought.content;
  el.innerHTML = `<span class="related-label">Related:</span> <span class="related-text">${escapeHtml(preview)}</span>`;
  el.style.display = '';
  // Expand capture window to show the hint row (+28px)
  SetCaptureHeight(88);
}

function dismissRelatedHint() {
  clearTimeout(relatedTimer);
  const el = document.getElementById('related-hint');
  if (el) el.style.display = 'none';
  // Restore normal capture height only if slash suggestions are also gone
  if (mode === 'braindump' && slashVisible.length === 0) {
    SetCaptureHeight(60);
  }
}

// ─── Garden (review mode) ─────────────────────────────────────────────────────

function mountGarden(container) {
  selectedThoughtId = null;
  reviewFilter = 'all';
  mergeSelectionIds = new Set();

  container.innerHTML = `
    <div id="review-layout">
      <aside id="sidebar">
        <nav id="sidebar-nav">
          <button class="nav-btn active" data-filter="all">All</button>
          <button class="nav-btn" data-filter="todo">To-do</button>
          <button class="nav-btn" data-filter="idea">Ideas</button>
          <button class="nav-btn" data-filter="calendar">Calendar</button>
          <button class="nav-btn" data-filter="reminder">Reminders</button>
          <button class="nav-btn actions-btn" data-filter="actions" style="display:none">
            Actions <span id="actions-count" class="actions-badge"></span>
          </button>
          <button class="nav-btn mishap-btn" data-filter="mishap" style="display:none">
            Review Needed <span id="mishap-count" class="mishap-badge"></span>
          </button>
        </nav>
      </aside>
      <div id="stream-wrapper">
        <div id="merge-toolbar" style="display:none">
          <span id="merge-count-label"></span>
          <button id="btn-merge-selected" class="btn-primary btn-merge">Merge selected</button>
          <button id="btn-merge-cancel" class="btn-ghost">Cancel</button>
        </div>
        <main id="stream"></main>
      </div>
      <aside id="inspector"></aside>
    </div>
  `;

  document.querySelectorAll('.nav-btn').forEach(btn => {
    btn.addEventListener('click', () => {
      document.querySelectorAll('.nav-btn').forEach(b => b.classList.remove('active'));
      btn.classList.add('active');
      reviewFilter = btn.dataset.filter;
      selectedThoughtId = null;
      mergeSelectionIds = new Set();
      document.getElementById('inspector').innerHTML = '';
      refreshGarden();
    });
  });

  document.getElementById('btn-merge-selected').addEventListener('click', async () => {
    const ids = Array.from(mergeSelectionIds);
    if (ids.length < 2) return;
    try {
      await MergeThoughts(ids);
      mergeSelectionIds = new Set();
      hideMergeToolbar();
      await refreshGarden();
    } catch (e) { console.error('MergeThoughts failed:', e); }
  });

  document.getElementById('btn-merge-cancel').addEventListener('click', () => {
    mergeSelectionIds = new Set();
    hideMergeToolbar();
    renderStream();
  });
}

async function refreshGarden() {
  // Refresh badges alongside the main stream
  refreshMishapBadge();
  refreshIntentsBadge();

  if (reviewFilter === 'mishap') {
    try {
      hiddenThoughts = await GetHiddenThoughts() || [];
      reviewThoughts = hiddenThoughts;
    } catch (e) {
      console.error('GetHiddenThoughts failed:', e);
      hiddenThoughts = [];
      reviewThoughts = [];
    }
  } else if (reviewFilter === 'actions') {
    try {
      pendingIntents = await GetPendingIntents() || [];
    } catch (e) {
      console.error('GetPendingIntents failed:', e);
      pendingIntents = [];
    }
  } else {
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
  }
  renderStream();
}

async function refreshMishapBadge() {
  try {
    hiddenThoughts = await GetHiddenThoughts() || [];
  } catch (_) {
    return;
  }
  const btn = document.querySelector('.mishap-btn');
  const badge = document.getElementById('mishap-count');
  if (!btn || !badge) return;
  const count = hiddenThoughts.length;
  if (count > 0) {
    btn.style.display = '';
    badge.textContent = String(count);
  } else {
    if (reviewFilter !== 'mishap') btn.style.display = 'none';
    badge.textContent = '';
  }
}

async function refreshIntentsBadge() {
  try {
    pendingIntents = await GetPendingIntents() || [];
  } catch (_) {
    return;
  }
  const btn = document.querySelector('.actions-btn');
  const badge = document.getElementById('actions-count');
  if (!btn || !badge) return;
  const count = pendingIntents.length;
  if (count > 0) {
    btn.style.display = '';
    badge.textContent = String(count);
  } else {
    if (reviewFilter !== 'actions') btn.style.display = 'none';
    badge.textContent = '';
  }
}

// ─── Wellbeing card (DELTA-5) ─────────────────────────────────────────────────

const WELLBEING_MESSAGES = [
  { text: 'Things feel heavy lately. Even a 2-minute walk can shift your state.', link: 'https://www.headspace.com', linkText: 'Try a breathing exercise' },
  { text: 'Noticing a pattern of difficult days. Being gentle with yourself matters.', link: 'https://www.calm.com', linkText: 'Take a moment to breathe' },
  { text: 'Your thoughts reflect a challenging period. Reaching out can help.', link: 'https://findahelpline.com', linkText: 'Find support near you' },
];

async function checkAndShowWellbeingCard() {
  // Check if user dismissed within the last 48h
  const dismissedUntil = localStorage.getItem('wellbeing_dismissed_until');
  if (dismissedUntil && Date.now() < Number(dismissedUntil)) return;

  try {
    const avg = await GetSentimentTrend(7);
    if (avg < -0.4) {
      showWellbeingCard();
    }
  } catch (_) { /* no-op */ }
}

function showWellbeingCard() {
  const stream = document.getElementById('stream');
  if (!stream) return;
  // Don't show if already displayed
  if (document.getElementById('wellbeing-card')) return;

  const msg = WELLBEING_MESSAGES[Math.floor(Math.random() * WELLBEING_MESSAGES.length)];
  const card = document.createElement('div');
  card.id = 'wellbeing-card';
  card.className = 'wellbeing-card';
  card.innerHTML = `
    <div class="wellbeing-text">${escapeHtml(msg.text)}</div>
    <div class="wellbeing-footer">
      <a class="wellbeing-link" href="#" data-href="${escapeHtml(msg.link)}">${escapeHtml(msg.linkText)}</a>
      <button class="wellbeing-dismiss">Dismiss for 48h</button>
    </div>
  `;
  stream.insertBefore(card, stream.firstChild);

  card.querySelector('.wellbeing-dismiss').addEventListener('click', () => {
    localStorage.setItem('wellbeing_dismissed_until', String(Date.now() + 48 * 60 * 60 * 1000));
    card.remove();
  });
}

// ─── Stream rendering ─────────────────────────────────────────────────────────

function renderStream() {
  const stream = document.getElementById('stream');
  if (!stream) return;

  if (reviewFilter === 'mishap') {
    renderMishapStream(stream);
    return;
  }

  if (reviewFilter === 'actions') {
    renderActionsStream(stream);
    return;
  }

  if (reviewThoughts.length === 0) {
    stream.innerHTML = `<div class="empty-state">No thoughts yet.<br>Start capturing with <kbd>Ctrl+Shift+Space</kbd>.</div>`;
    return;
  }

  const query = document.getElementById('thought-input')?.value.trim() || '';
  const showCheckboxes = mergeSelectionIds.size > 0;

  stream.innerHTML = reviewThoughts.map(t => {
    const content = query ? highlight(t.content, query) : escapeHtml(t.content);
    const isSelected = t.id === selectedThoughtId;
    const isChecked = mergeSelectionIds.has(t.id);
    return `
      <div class="stream-item${isSelected ? ' selected' : ''}${isChecked ? ' merge-checked' : ''}" data-id="${t.id}">
        <label class="merge-checkbox-wrap" title="Select for merge">
          <input type="checkbox" class="merge-checkbox" data-id="${t.id}"${isChecked ? ' checked' : ''}>
        </label>
        <div class="stream-body">
          <div class="stream-content">${content}</div>
          <div class="stream-footer">
            ${renderTags(t.tags || [])}
            <span class="stream-time">${formatRelativeTime(t.created_at)}</span>
          </div>
        </div>
      </div>
    `;
  }).join('');

  stream.querySelectorAll('.stream-item').forEach(el => {
    el.addEventListener('click', (e) => {
      if (e.target.closest('.merge-checkbox-wrap')) return; // handled below
      selectThought(Number(el.dataset.id));
    });
  });

  stream.querySelectorAll('.merge-checkbox').forEach(cb => {
    cb.addEventListener('change', () => {
      const id = Number(cb.dataset.id);
      if (cb.checked) {
        mergeSelectionIds.add(id);
      } else {
        mergeSelectionIds.delete(id);
      }
      updateMergeToolbar();
      renderStream();
    });
  });

  // Restore wellbeing card after re-render if trend warrants it
  checkAndShowWellbeingCard();
}

function updateMergeToolbar() {
  const toolbar = document.getElementById('merge-toolbar');
  const label = document.getElementById('merge-count-label');
  if (!toolbar || !label) return;
  if (mergeSelectionIds.size > 0) {
    toolbar.style.display = 'flex';
    label.textContent = `${mergeSelectionIds.size} selected`;
  } else {
    toolbar.style.display = 'none';
    label.textContent = '';
  }
}

function hideMergeToolbar() {
  const toolbar = document.getElementById('merge-toolbar');
  if (toolbar) toolbar.style.display = 'none';
}

function renderMishapStream(stream) {
  if (hiddenThoughts.length === 0) {
    stream.innerHTML = `<div class="empty-state">Nothing here.<br>All accidental captures have been reviewed.</div>`;
    return;
  }

  stream.innerHTML = hiddenThoughts.map(t => `
    <div class="mishap-card" data-id="${t.id}">
      <div class="mishap-content">${escapeHtml(t.content)}</div>
      <div class="mishap-footer">
        <span class="stream-time">${formatRelativeTime(t.created_at)}</span>
        <div class="mishap-actions">
          <button class="btn-keep" data-id="${t.id}">Keep</button>
          <button class="btn-delete-mishap" data-id="${t.id}">Delete</button>
        </div>
      </div>
    </div>
  `).join('');

  stream.querySelectorAll('.btn-keep').forEach(btn => {
    btn.addEventListener('click', async () => {
      const id = Number(btn.dataset.id);
      try {
        await UnhideThought(id);
        hiddenThoughts = hiddenThoughts.filter(t => t.id !== id);
        reviewThoughts = hiddenThoughts;
        refreshMishapBadge();
        renderStream();
      } catch (e) { console.error(e); }
    });
  });

  stream.querySelectorAll('.btn-delete-mishap').forEach(btn => {
    let armed = false;
    btn.addEventListener('click', async () => {
      const id = Number(btn.dataset.id);
      if (!armed) {
        armed = true;
        btn.textContent = 'Confirm?';
        setTimeout(() => {
          armed = false;
          if (btn.isConnected) btn.textContent = 'Delete';
        }, 3000);
        return;
      }
      try {
        await DeleteThought(id);
        hiddenThoughts = hiddenThoughts.filter(t => t.id !== id);
        reviewThoughts = hiddenThoughts;
        refreshMishapBadge();
        renderStream();
      } catch (e) { console.error(e); }
    });
  });
}

// ─── Intent Actions stream (DELTA-4) ─────────────────────────────────────────

const INTENT_ICONS = { calendar: '📅', task: '✅', reminder: '🔔' };

function renderActionsStream(stream) {
  if (pendingIntents.length === 0) {
    stream.innerHTML = `<div class="empty-state">No pending actions.<br>Intents are detected automatically when you capture thoughts.</div>`;
    return;
  }

  stream.innerHTML = pendingIntents.map(intent => `
    <div class="intent-card" data-id="${intent.id}">
      <div class="intent-header">
        <span class="intent-icon">${INTENT_ICONS[intent.type] || '•'}</span>
        <span class="intent-type">${escapeHtml(intent.type)}</span>
        <span class="intent-time">${formatRelativeTime(intent.created_at)}</span>
      </div>
      <div class="intent-title" contenteditable="false" data-id="${intent.id}">${escapeHtml(intent.title)}</div>
      <div class="intent-actions">
        <button class="btn-confirm-intent" data-id="${intent.id}">Confirm</button>
        <button class="btn-edit-intent" data-id="${intent.id}">Edit</button>
        <button class="btn-dismiss-intent" data-id="${intent.id}">Dismiss</button>
      </div>
    </div>
  `).join('');

  stream.querySelectorAll('.btn-confirm-intent').forEach(btn => {
    btn.addEventListener('click', async () => {
      const id = btn.dataset.id;
      try {
        await ConfirmIntent(id);
        pendingIntents = pendingIntents.filter(i => i.id !== id);
        refreshIntentsBadge();
        renderActionsStream(stream);
      } catch (e) { console.error('ConfirmIntent failed:', e); }
    });
  });

  stream.querySelectorAll('.btn-edit-intent').forEach(btn => {
    btn.addEventListener('click', () => {
      const id = btn.dataset.id;
      const titleEl = stream.querySelector(`.intent-title[data-id="${id}"]`);
      if (!titleEl) return;
      titleEl.contentEditable = 'true';
      titleEl.focus();
      // Place cursor at end
      const range = document.createRange();
      range.selectNodeContents(titleEl);
      range.collapse(false);
      const sel = window.getSelection();
      sel.removeAllRanges();
      sel.addRange(range);
      btn.textContent = 'Done';
      btn.classList.add('btn-edit-done');
      btn.removeEventListener('click', arguments.callee);
      btn.addEventListener('click', () => {
        titleEl.contentEditable = 'false';
        // Update in local state (DB intent title is not updated — display only)
        const intent = pendingIntents.find(i => i.id === id);
        if (intent) intent.title = titleEl.textContent.trim() || intent.title;
        btn.textContent = 'Edit';
        btn.classList.remove('btn-edit-done');
      });
    });
  });

  stream.querySelectorAll('.btn-dismiss-intent').forEach(btn => {
    btn.addEventListener('click', async () => {
      const id = btn.dataset.id;
      try {
        await DismissIntent(id);
        pendingIntents = pendingIntents.filter(i => i.id !== id);
        refreshIntentsBadge();
        renderActionsStream(stream);
      } catch (e) { console.error('DismissIntent failed:', e); }
    });
  });
}

// ─── Thought navigation & selection ──────────────────────────────────────────

function navigateThoughts(delta) {
  if (reviewThoughts.length === 0) return;
  const cur = reviewThoughts.findIndex(t => t.id === selectedThoughtId);
  const next = cur === -1
    ? (delta > 0 ? 0 : reviewThoughts.length - 1)
    : Math.max(0, Math.min(reviewThoughts.length - 1, cur + delta));
  selectThought(reviewThoughts[next].id);
}

async function selectThought(id) {
  selectedThoughtId = id;
  renderStream();
  document.querySelector('.stream-item.selected')?.scrollIntoView({ block: 'nearest' });
  try {
    const t = await GetThought(id);
    renderInspector(t);
  } catch (_) {
    renderInspector(null);
  }
}

// ─── Inspector ────────────────────────────────────────────────────────────────

function renderInspector(t) {
  const inspector = document.getElementById('inspector');
  if (!inspector) return;
  if (!t) { inspector.innerHTML = ''; return; }

  const hasEdits = t.content !== t.raw_content;

  const shadowToggle = hasEdits ? `
    <div class="inspector-section">
      <button id="btn-shadow-toggle" class="btn-shadow-toggle">View original</button>
      <div id="shadow-diff" style="display:none" class="shadow-diff"></div>
    </div>
  ` : '';

  const hasContext = t.context && (t.context.app_name || t.context.window_title || t.context.url);
  const contextSection = `
    <div class="inspector-section">
      <label class="inspector-label">Captured from</label>
      <div class="context-info">
        ${hasContext ? `
          ${t.context.app_name ? `<span class="ctx-app">${escapeHtml(t.context.app_name)}</span>` : ''}
          ${t.context.window_title ? `<span class="ctx-window">${escapeHtml(t.context.window_title)}</span>` : ''}
          ${t.context.url ? `<span class="ctx-window">${escapeHtml(t.context.url)}</span>` : ''}
        ` : '<span class="ctx-empty">—</span>'}
      </div>
    </div>
  `;

  // Show merged_from if this is a merged thought
  const mergedFrom = t.meta && t.meta.merged_from ? t.meta.merged_from : null;
  const mergedSection = mergedFrom ? `
    <div class="inspector-section">
      <label class="inspector-label">Merged from</label>
      <div class="ctx-window">${mergedFrom.length} thoughts (IDs: ${mergedFrom.join(', ')})</div>
    </div>
  ` : '';

  inspector.innerHTML = `
    <div id="inspector-inner">
      <div class="inspector-section">
        <div class="inspector-label-row">
          <label class="inspector-label">Content</label>
          <button id="btn-clean-toggle" class="btn-clean-toggle">Clean view</button>
        </div>
        <textarea id="inspector-content" rows="6">${escapeHtml(t.content)}</textarea>
      </div>
      ${shadowToggle}
      <div class="inspector-section">
        <label class="inspector-label">Tags</label>
        <div>${renderTags(t.tags || [])}</div>
      </div>
      ${contextSection}
      ${mergedSection}
      <div class="inspector-actions">
        <button id="btn-save-edit" class="btn-primary">Save</button>
        <button id="btn-delete" class="btn-danger">Delete</button>
      </div>
      <div class="inspector-times">
        <span>Created ${formatAbsoluteTime(t.created_at)}</span>
      </div>
    </div>
  `;

  if (hasEdits) {
    let diffVisible = false;
    const toggleBtn = document.getElementById('btn-shadow-toggle');
    const diffEl = document.getElementById('shadow-diff');
    toggleBtn.addEventListener('click', () => {
      diffVisible = !diffVisible;
      if (diffVisible) {
        diffEl.innerHTML = renderWordDiff(t.raw_content, t.content);
        diffEl.style.display = '';
        toggleBtn.textContent = 'Hide original';
      } else {
        diffEl.style.display = 'none';
        toggleBtn.textContent = 'View original';
      }
    });
  }

  // DELTA-7c: Clean View toggle
  let cleanViewActive = false;
  const cleanBtn = document.getElementById('btn-clean-toggle');
  const contentArea = document.getElementById('inspector-content');
  cleanBtn.addEventListener('click', async () => {
    cleanViewActive = !cleanViewActive;
    if (cleanViewActive) {
      cleanBtn.classList.add('active');
      cleanBtn.textContent = 'Clean view ✓';
      contentArea.disabled = true;
      contentArea.style.opacity = '0.6';
      try {
        const cleaned = await CleanText(t.id);
        contentArea.value = cleaned;
      } catch (_) {
        cleanViewActive = false;
        cleanBtn.classList.remove('active');
        cleanBtn.textContent = 'Clean view';
        contentArea.disabled = false;
        contentArea.style.opacity = '';
      }
    } else {
      cleanBtn.classList.remove('active');
      cleanBtn.textContent = 'Clean view';
      contentArea.value = t.content;
      contentArea.disabled = false;
      contentArea.style.opacity = '';
    }
  });

  document.getElementById('btn-save-edit').addEventListener('click', async () => {
    if (cleanViewActive) return; // don't save cleaned text
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

  const deleteBtn = document.getElementById('btn-delete');
  let deleteArmed = false;
  deleteBtn.addEventListener('click', async () => {
    if (!deleteArmed) {
      deleteArmed = true;
      deleteBtn.textContent = 'Confirm delete';
      deleteBtn.classList.add('armed');
      setTimeout(() => {
        deleteArmed = false;
        if (deleteBtn.isConnected) {
          deleteBtn.textContent = 'Delete';
          deleteBtn.classList.remove('armed');
        }
      }, 3000);
      return;
    }
    try {
      await DeleteThought(t.id);
      selectedThoughtId = null;
      inspector.innerHTML = '';
      reviewThoughts = reviewThoughts.filter(x => x.id !== t.id);
      renderStream();
    } catch (e) { console.error(e); }
  });
}

// ─── Word-level diff ──────────────────────────────────────────────────────────

function renderWordDiff(original, modified) {
  const a = original.split(/(\s+)/);
  const b = modified.split(/(\s+)/);
  const m = a.length, n = b.length;

  // LCS table
  const dp = Array.from({length: m + 1}, () => new Int32Array(n + 1));
  for (let i = 1; i <= m; i++)
    for (let j = 1; j <= n; j++)
      dp[i][j] = a[i-1] === b[j-1] ? dp[i-1][j-1] + 1 : Math.max(dp[i-1][j], dp[i][j-1]);

  // Backtrack
  const ops = [];
  let i = m, j = n;
  while (i > 0 || j > 0) {
    if (i > 0 && j > 0 && a[i-1] === b[j-1]) { ops.unshift({t: '=', v: a[i-1]}); i--; j--; }
    else if (j > 0 && (i === 0 || dp[i][j-1] >= dp[i-1][j])) { ops.unshift({t: '+', v: b[j-1]}); j--; }
    else { ops.unshift({t: '-', v: a[i-1]}); i--; }
  }

  return ops.map(op => {
    const s = escapeHtml(op.v);
    if (op.t === '-') return `<span class="diff-remove">${s}</span>`;
    if (op.t === '+') return `<span class="diff-add">${s}</span>`;
    return s;
  }).join('');
}

// ─── Slash completion ─────────────────────────────────────────────────────────

function updateSlashSuggestions(value) {
  const query = value.toLowerCase();
  const prev = slashVisible;
  slashVisible = SLASH_COMMANDS.filter(c => c.cmd.startsWith(query));
  if (slashVisible.length === 0) {
    dismissSlash();
    return;
  }
  // Keep current index in range; reset to -1 (no selection) when the list first appears or shrinks past it
  if (prev.length === 0 || slashIndex >= slashVisible.length) slashIndex = -1;
  renderSlashSuggestions();
  if (mode === 'braindump') {
    SetCaptureHeight(60 + slashVisible.length * 48);
  }
}

function renderSlashSuggestions() {
  const el = document.getElementById('slash-suggestions');
  if (!el) return;
  el.style.display = '';
  el.innerHTML = slashVisible.map((c, i) => `
    <div class="slash-item${i === slashIndex ? ' active' : ''}" data-index="${i}">
      <span class="slash-cmd">${escapeHtml(c.cmd)}</span>
      <span class="slash-desc">${escapeHtml(c.description)}</span>
    </div>
  `).join('');
  el.querySelectorAll('.slash-item').forEach(row => {
    row.addEventListener('mousedown', e => {
      e.preventDefault(); // prevent blur before click registers
      applySlashCommand(slashVisible[Number(row.dataset.index)].cmd);
    });
    row.addEventListener('mouseenter', () => {
      slashIndex = Number(row.dataset.index);
      renderSlashSuggestions();
    });
  });
}

function dismissSlash() {
  slashVisible = [];
  slashIndex = -1;
  const el = document.getElementById('slash-suggestions');
  if (el) el.style.display = 'none';
  if (mode === 'braindump') SetCaptureHeight(60);
}

function applySlashCommand(cmd) {
  const input = document.getElementById('thought-input');
  if (input) { input.value = cmd; input.focus(); }
  dismissSlash();
  // Execute immediately
  if (cmd === '/review') {
    if (input) input.value = '';
    ShowReview();
  }
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
