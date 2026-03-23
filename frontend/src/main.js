import './style.css';
import { setupEscHandler } from './esc-handler.js';
import { setupTabHandler } from './tab-handler.js';
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
  ExportJSON,
  ExportCSV,
  ImportJSON,
  ImportCSV,
  GetSettings,
  SaveSettings,
  Quit,
  RestartApp,
} from '../bindings/github.com/thawts/thawts/internal/app/app.js';
import { Events } from '@wailsio/runtime';

// ─── State ────────────────────────────────────────────────────────────────────

let mode = 'braindump'; // 'braindump' | 'review'
let reviewThoughts = [];
let hiddenThoughts = [];
let pendingIntents = [];
let selectedThoughtId = null;
let reviewFilter = 'all'; // 'all' | tag name | 'mishap' | 'actions'
let searchTimer = null;
let relatedTimer = null;  // debounce for proactive synthesis (DELTA-3c)
let deleteArmedTimer = null; // armed state for keyboard delete (d d)
let pendingEdit = false;    // set when 'e' is pressed before inspector finishes loading
let mergeSelectionIds = new Set(); // IDs selected for merge (DELTA-7a)
let cmdSelectedIndex = -1; // selected index in slash-command palette
let settingsOpen = false;   // true while settings panel is shown
let pendingSettings = false; // true when settings requested from braindump mode
let escOverride = null;      // function to call instead of normal Escape chain

const SLASH_COMMANDS = [
  { id: 'settings',            label: '/settings',            desc: 'Open settings (hotkeys, startup)' },
  { id: 'export-json',         label: '/export json',         desc: 'Export all thoughts to JSON' },
  { id: 'export-csv',          label: '/export csv',          desc: 'Export all thoughts to CSV' },
  { id: 'import-json',         label: '/import json',         desc: 'Import thoughts from JSON (additive)' },
  { id: 'import-json-restore', label: '/import json restore', desc: 'Import JSON and replace all existing data' },
  { id: 'import-csv',          label: '/import csv',          desc: 'Import thoughts from CSV (additive)' },
  { id: 'import-csv-restore',  label: '/import csv restore',  desc: 'Import CSV and replace all existing data' },
  { id: 'restart',             label: '/restart',             desc: 'Restart the application' },
  { id: 'quit',                label: '/quit',                desc: 'Quit the application' },
];

// ─── Bootstrap ───────────────────────────────────────────────────────────────

document.addEventListener('DOMContentLoaded', () => {
  buildApp();
  setupEscHandler({
    getMode:         () => mode,
    getEscOverride:  () => escOverride,
    getSelectedId:   () => selectedThoughtId,
    deselectThought: () => {
      clearDeleteArmed();
      pendingEdit = false;
      selectedThoughtId = null;
      renderStream();
      const inspector = document.getElementById('inspector');
      if (inspector) inspector.innerHTML = '';
    },
    getInputValue:   () => document.getElementById('thought-input')?.value ?? '',
    clearInput:      () => {
      const el = document.getElementById('thought-input');
      if (el) { el.value = ''; onInputChange(); }
    },
    focusInput:      () => document.getElementById('thought-input')?.focus(),
    hideWindow:      HideWindow,
    showCapture:     ShowCapture,
  });
  setupTabHandler(() => mode, ShowReview, ShowCapture);

  // Navigation and selection shortcuts in review mode.
  // Uses capture so they fire regardless of what element has focus.
  document.addEventListener('keydown', (e) => {
    if (mode !== 'review') return;
    // Skip if focus is in the inspector editor — that input handles its own keys.
    if (e.target?.id === 'inspector-content') return;

    // Arrow navigation works regardless of search text,
    // but yields to the slash-command palette when it is open.
    if (reviewFilter !== 'mishap' && reviewFilter !== 'actions' && !isCmdPaletteVisible()) {
      if (e.key === 'ArrowDown') { e.preventDefault(); navigateThoughts(1); return; }
      if (e.key === 'ArrowUp')   { e.preventDefault(); navigateThoughts(-1); return; }
    }

    if (selectedThoughtId === null) return;
    if (e.key === 'd') { e.preventDefault(); handleDeleteKey(); }
    else if (e.key === 'e') { e.preventDefault(); handleEditKey(); }
  }, { capture: true });


});

Events.On('mode:capture', () => enterBraindump());
Events.On('mode:review',  () => enterReview());
Events.On('thought:classified', () => { if (mode === 'review') refreshGarden(); });
Events.On('mishaps:changed', () => { if (mode === 'review') refreshMishapBadge(); });
Events.On('intents:changed', () => { if (mode === 'review') refreshIntentsBadge(); });
Events.On('thoughts:merged', () => { if (mode === 'review') refreshGarden(); });
Events.On('wellbeing:alert', () => { if (mode === 'review') checkAndShowWellbeingCard(); });
Events.On('thoughts:imported', () => { if (mode === 'review') refreshGarden(); });

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
      <div id="cmd-palette" style="display:none" role="listbox"></div>
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
  clearDeleteArmed();
  dismissCmdPalette();
  dismissRelatedHint();
  document.getElementById('shell').dataset.mode = 'braindump';
  document.getElementById('garden-area').style.display = 'none';
  document.getElementById('garden-area').innerHTML = '';
  const input = document.getElementById('thought-input');
  if (input) { input.placeholder = 'What\'s on your mind…'; setTimeout(() => input.focus(), 50); }
  SetCaptureHeight(60);
}

function enterReview() {
  mode = 'review';
  dismissRelatedHint(); // cancel any pending braindump-mode related-hint timer
  document.getElementById('shell').dataset.mode = 'review';
  const gardenArea = document.getElementById('garden-area');
  gardenArea.style.display = 'flex';
  const input = document.getElementById('thought-input');
  if (input) { input.placeholder = 'Search garden…'; setTimeout(() => input.focus(), 50); }
  if (pendingSettings) {
    pendingSettings = false;
    showSettings();
    return;
  }
  mountGarden(gardenArea);
  refreshGarden();
  checkAndShowWellbeingCard();
}

// ─── Input ────────────────────────────────────────────────────────────────────

async function onInputKeydown(e) {
  const input = e.target;
  const text = input.value.trim();

  // Slash command palette navigation
  if (isCmdPaletteVisible()) {
    if (e.key === 'ArrowDown') {
      e.preventDefault();
      moveCmdSelection(1);
      return;
    }
    if (e.key === 'ArrowUp') {
      e.preventDefault();
      moveCmdSelection(-1);
      return;
    }
    if (e.key === 'Escape') {
      e.preventDefault();
      dismissCmdPalette();
      return;
    }
    if (e.key === 'Enter') {
      e.preventDefault();
      const cmds = getFilteredCmds(text);
      const idx = cmdSelectedIndex >= 0 ? cmdSelectedIndex : (cmds.length === 1 ? 0 : -1);
      if (idx >= 0 && idx < cmds.length) {
        input.value = '';
        dismissCmdPalette();
        await executeSlashCommand(cmds[idx].id);
      }
      return;
    }
    if (e.key === 'Tab') {
      // Autocomplete to the selected / first command label
      const cmds = getFilteredCmds(text);
      const idx = cmdSelectedIndex >= 0 ? cmdSelectedIndex : 0;
      if (cmds[idx]) {
        e.preventDefault();
        input.value = cmds[idx].label;
        updateCmdPalette(cmds[idx].label);
      }
      return;
    }
  }

  if (e.key === 'Enter') {
    e.preventDefault();
    if (mode === 'braindump' || mode === 'review') {
      if (!text) return;
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

  // Slash command palette — active in both modes.
  if (val.startsWith('/')) {
    updateCmdPalette(val);
    return; // don't search or show related hint while in command mode
  }
  dismissCmdPalette();

  if (mode === 'review') {
    clearTimeout(searchTimer);
    searchTimer = setTimeout(refreshGarden, 250);
  }

  // Proactive synthesis — show a related memory hint after 1.5s of pause.
  if (mode === 'braindump' && val.trim().length >= 3) {
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
  if (mode === 'braindump' && !isCmdPaletteVisible()) {
    SetCaptureHeight(60);
  }
}

// ─── Slash command palette ────────────────────────────────────────────────────

function getFilteredCmds(val) {
  const q = val.toLowerCase().trim();
  if (!q || q === '/') return SLASH_COMMANDS;
  return SLASH_COMMANDS.filter(c => c.label.startsWith(q) || c.label.includes(q));
}

function isCmdPaletteVisible() {
  const el = document.getElementById('cmd-palette');
  return el && el.style.display !== 'none';
}

function updateCmdPalette(val) {
  const el = document.getElementById('cmd-palette');
  if (!el) return;
  const cmds = getFilteredCmds(val);
  if (!cmds.length) {
    dismissCmdPalette();
    return;
  }
  // Clamp selection
  if (cmdSelectedIndex >= cmds.length) cmdSelectedIndex = cmds.length - 1;
  el.innerHTML = cmds.map((c, i) => `
    <div class="cmd-item${i === cmdSelectedIndex ? ' cmd-item--selected' : ''}" data-idx="${i}" role="option">
      <span class="cmd-label">${escapeHtml(c.label)}</span>
      <span class="cmd-desc">${escapeHtml(c.desc)}</span>
    </div>
  `).join('');
  el.style.display = '';
  el.querySelectorAll('.cmd-item').forEach(item => {
    item.addEventListener('click', async () => {
      const idx = parseInt(item.dataset.idx, 10);
      document.getElementById('thought-input').value = '';
      dismissCmdPalette();
      await executeSlashCommand(cmds[idx].id);
    });
  });
  // Expand capture window to show palette rows (each ~32px)
  if (mode === 'braindump') {
    SetCaptureHeight(60 + cmds.length * 36);
  }
}

function dismissCmdPalette() {
  const el = document.getElementById('cmd-palette');
  if (el) el.style.display = 'none';
  cmdSelectedIndex = -1;
  if (mode === 'braindump') SetCaptureHeight(60);
}

function moveCmdSelection(delta) {
  const val = document.getElementById('thought-input')?.value ?? '';
  const cmds = getFilteredCmds(val);
  if (!cmds.length) return;
  cmdSelectedIndex = (cmdSelectedIndex + delta + cmds.length) % cmds.length;
  updateCmdPalette(val);
}

async function executeSlashCommand(id) {
  const input = document.getElementById('thought-input');
  const showResult = (msg, isError) => {
    if (!msg) return;
    const el = document.getElementById('related-hint');
    if (!el) return;
    el.innerHTML = `<span class="${isError ? 'cmd-error' : 'cmd-ok'}">${escapeHtml(msg)}</span>`;
    el.style.display = '';
    if (mode === 'braindump') SetCaptureHeight(88);
    setTimeout(() => dismissRelatedHint(), isError ? 5000 : 3000);
  };
  try {
    let result = '';
    switch (id) {
      case 'settings':
        if (mode !== 'review') {
          pendingSettings = true;
          await ShowReview();
        } else {
          showSettings();
        }
        return;
      case 'export-json':        result = await ExportJSON(); break;
      case 'export-csv':         result = await ExportCSV(); break;
      case 'import-json':        result = await ImportJSON(false); break;
      case 'import-json-restore':result = await ImportJSON(true); break;
      case 'import-csv':         result = await ImportCSV(false); break;
      case 'import-csv-restore': result = await ImportCSV(true); break;
      case 'restart':            RestartApp(); return;
      case 'quit':               Quit(); return;
    }
    showResult(result, false);
    if (id.startsWith('import') && mode === 'review') await refreshGarden();
  } catch (err) {
    console.error('slash command failed:', err);
    showResult(String(err), true);
  }
  if (input) input.focus();
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
  clearDeleteArmed();
  if (reviewThoughts.length === 0) return;
  const cur = reviewThoughts.findIndex(t => t.id === selectedThoughtId);
  const next = cur === -1
    ? (delta > 0 ? 0 : reviewThoughts.length - 1)
    : Math.max(0, Math.min(reviewThoughts.length - 1, cur + delta));
  selectThought(reviewThoughts[next].id);
}

async function selectThought(id) {
  clearDeleteArmed();
  selectedThoughtId = id;
  renderStream();
  document.querySelector('.stream-item.selected')?.scrollIntoView({ block: 'nearest' });
  document.getElementById('thought-input')?.blur();
  try {
    const t = await GetThought(id);
    renderInspector(t);
    if (pendingEdit) {
      pendingEdit = false;
      focusInspectorContent();
    }
  } catch (_) {
    pendingEdit = false;
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
        <input id="inspector-content" type="text" value="${escapeHtml(t.content)}" />
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

  async function saveInspectorEdit() {
    if (cleanViewActive) return;
    const newContent = document.getElementById('inspector-content').value.trim();
    if (!newContent || newContent === t.content) {
      document.getElementById('inspector-content')?.blur();
      return;
    }
    try {
      const updated = await UpdateThought(t.id, newContent);
      const idx = reviewThoughts.findIndex(x => x.id === t.id);
      if (idx !== -1) reviewThoughts[idx] = updated;
      renderStream();
      renderInspector(updated);
    } catch (e) { console.error(e); }
  }

  document.getElementById('btn-save-edit').addEventListener('click', saveInspectorEdit);

  document.getElementById('inspector-content').addEventListener('keydown', async (e) => {
    if (e.key === 'Enter') { e.preventDefault(); await saveInspectorEdit(); }
    if (e.key === 'Escape') {
      e.preventDefault();
      e.stopPropagation(); // prevent esc-handler from switching modes
      e.target.value = t.content; // discard changes
      e.target.blur();
    }
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

// ─── Keyboard shortcuts: delete and edit ─────────────────────────────────────

function clearDeleteArmed() {
  if (deleteArmedTimer) {
    clearTimeout(deleteArmedTimer);
    deleteArmedTimer = null;
  }
  document.querySelector('.stream-item.delete-armed')?.classList.remove('delete-armed');
}

function handleDeleteKey() {
  const item = document.querySelector('.stream-item.selected');
  if (!item) return;
  if (deleteArmedTimer) {
    // Second d: confirm
    clearDeleteArmed();
    confirmDeleteThought(selectedThoughtId);
  } else {
    // First d: arm
    item.classList.add('delete-armed');
    deleteArmedTimer = setTimeout(() => clearDeleteArmed(), 3000);
  }
}

async function confirmDeleteThought(id) {
  try {
    await DeleteThought(id);
    selectedThoughtId = null;
    document.getElementById('inspector').innerHTML = '';
    reviewThoughts = reviewThoughts.filter(t => t.id !== id);
    renderStream();
  } catch (e) { console.error('DeleteThought failed:', e); }
}

function focusInspectorContent() {
  const el = document.getElementById('inspector-content');
  if (!el) return;
  el.focus();
  el.selectionStart = el.selectionEnd = el.value.length;
}

function handleEditKey() {
  const el = document.getElementById('inspector-content');
  if (el) {
    focusInspectorContent();
  } else {
    // Inspector is still loading (async GetThought); focus it once it's rendered.
    pendingEdit = true;
  }
}

// ─── Settings ────────────────────────────────────────────────────────────────

const isMac = navigator.platform.toUpperCase().includes('MAC');

async function showSettings() {
  settingsOpen = true;
  escOverride = closeSettings;
  const gardenArea = document.getElementById('garden-area');
  gardenArea.style.display = 'flex';
  gardenArea.innerHTML = `<div id="settings-panel"><div class="settings-loading">Loading…</div></div>`;

  let settings;
  try {
    settings = await GetSettings();
  } catch (e) {
    gardenArea.innerHTML = `<div id="settings-panel"><p class="settings-error">Failed to load settings: ${escapeHtml(String(e))}</p></div>`;
    return;
  }
  renderSettingsPanel(gardenArea, settings);
}

function closeSettings() {
  settingsOpen = false;
  escOverride = null;
  const gardenArea = document.getElementById('garden-area');
  mountGarden(gardenArea);
  refreshGarden();
}

function renderSettingsPanel(container, settings) {
  let captureHotkey = settings.capture_hotkey || (isMac ? 'ctrl+option+space' : 'ctrl+alt+space');
  let reviewHotkey  = settings.review_hotkey  || (isMac ? 'cmd+option+r' : '');

  const reviewRow = isMac ? `
    <div class="settings-row">
      <div class="settings-row-label">
        <span class="settings-label">Review hotkey</span>
        <span class="settings-hint">macOS only</span>
      </div>
      <div class="hotkey-recorder" id="hotkey-review" tabindex="0" title="Click to record" data-hotkey="${escapeHtml(reviewHotkey)}">${formatHotkey(reviewHotkey)}</div>
    </div>
  ` : '';

  container.innerHTML = `
    <div id="settings-panel">
      <div class="settings-section">
        <div class="settings-section-title">Keyboard shortcuts</div>
        <div class="settings-row">
          <div class="settings-row-label">
            <span class="settings-label">Capture hotkey</span>
          </div>
          <div class="hotkey-recorder" id="hotkey-capture" tabindex="0" title="Click to record" data-hotkey="${escapeHtml(captureHotkey)}">${formatHotkey(captureHotkey)}</div>
        </div>
        ${reviewRow}
      </div>
      <div class="settings-section">
        <div class="settings-section-title">Startup</div>
        <div class="settings-row">
          <div class="settings-row-label">
            <span class="settings-label">Launch at login</span>
          </div>
          <label class="settings-toggle">
            <input type="checkbox" id="launch-at-login" ${settings.launch_at_login ? 'checked' : ''}>
            <span class="toggle-slider"></span>
          </label>
        </div>
      </div>
      <div class="settings-footer">
        <button id="btn-close-settings" class="btn-ghost">Done</button>
        <span id="settings-status" class="settings-status"></span>
      </div>
    </div>
  `;

  let statusTimer = null;
  function showStatus(msg, isError) {
    const statusEl = document.getElementById('settings-status');
    if (!statusEl) return;
    clearTimeout(statusTimer);
    statusEl.textContent = msg;
    statusEl.className = `settings-status ${isError ? 'error' : 'ok'}`;
    statusTimer = setTimeout(() => {
      statusEl.textContent = '';
      statusEl.className = 'settings-status';
    }, isError ? 5000 : 3000);
  }

  async function persist() {
    try {
      await SaveSettings({
        capture_hotkey:  captureHotkey,
        review_hotkey:   reviewHotkey,
        launch_at_login: document.getElementById('launch-at-login').checked,
      });
      showStatus('Saved.', false);
    } catch (e) {
      showStatus(String(e), true);
    }
  }

  const recorderOpts = {
    onRecordingStart: (cancelFn) => { escOverride = cancelFn; },
    onRecordingEnd:   ()         => { escOverride = closeSettings; },
  };
  setupHotkeyRecorder(document.getElementById('hotkey-capture'), (hk) => {
    captureHotkey = hk;
    persist();
  }, recorderOpts);
  if (isMac) {
    setupHotkeyRecorder(document.getElementById('hotkey-review'), (hk) => {
      reviewHotkey = hk;
      persist();
    }, recorderOpts);
  }

  document.getElementById('launch-at-login').addEventListener('change', () => persist());
  document.getElementById('btn-close-settings').addEventListener('click', closeSettings);
}

function setupHotkeyRecorder(el, onRecord, { onRecordingStart, onRecordingEnd } = {}) {
  if (!el) return;

  el.addEventListener('click', startRecording);
  el.addEventListener('keydown', (e) => { if (e.key === 'Enter' || e.key === ' ') startRecording(); });

  function startRecording() {
    const previous = el.dataset.hotkey || '';
    el.textContent = 'Press a key combo…';
    el.classList.add('recording');
    if (onRecordingStart) onRecordingStart(cancel);

    function onKeydown(e) {
      const modifiers = ['Control', 'Alt', 'Meta', 'Shift'];
      if (modifiers.includes(e.key)) return; // modifier-only press, keep waiting

      if (e.key === 'Escape') {
        e.preventDefault();
        e.stopPropagation();
        cancel();
        return;
      }

      if (!e.ctrlKey && !e.altKey && !e.metaKey && !e.shiftKey) return; // must have modifier

      const key = codeToKey(e.code);
      if (!key) return;

      e.preventDefault();
      e.stopPropagation();

      const parts = [];
      if (e.ctrlKey) parts.push('ctrl');
      if (e.altKey)  parts.push(isMac ? 'option' : 'alt');
      if (e.metaKey) parts.push('cmd');
      if (e.shiftKey) parts.push('shift');
      parts.push(key);

      const hk = parts.join('+');
      el.dataset.hotkey = hk;
      el.textContent = formatHotkey(hk);
      el.classList.remove('recording');
      document.removeEventListener('keydown', onKeydown, true);
      if (onRecordingEnd) onRecordingEnd();
      onRecord(hk);
    }

    function cancel() {
      el.classList.remove('recording');
      el.textContent = formatHotkey(previous) || '—';
      document.removeEventListener('keydown', onKeydown, true);
      if (onRecordingEnd) onRecordingEnd();
    }

    document.addEventListener('keydown', onKeydown, true);
  }
}

function codeToKey(code) {
  if (code === 'Space') return 'space';
  if (code.startsWith('Key') && code.length === 4) return code[3].toLowerCase();
  return null;
}

function formatHotkey(str) {
  if (!str) return '—';
  const parts = str.split('+');
  if (isMac) {
    return parts.map(p => {
      switch (p.toLowerCase()) {
        case 'ctrl':            return '⌃';
        case 'option': case 'alt': return '⌥';
        case 'cmd':             return '⌘';
        case 'shift':           return '⇧';
        case 'space':           return 'Space';
        default:                return p.toUpperCase();
      }
    }).join('');
  }
  return parts.map(p => {
    switch (p.toLowerCase()) {
      case 'ctrl':                    return 'Ctrl';
      case 'alt': case 'option':      return 'Alt';
      case 'cmd': case 'win':         return 'Win';
      case 'shift':                   return 'Shift';
      case 'space':                   return 'Space';
      default:                        return p.toUpperCase();
    }
  }).join('+');
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
