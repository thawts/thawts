import './style.css';
import { Hide, Save, Search, SetWindowHeight } from '../wailsjs/go/app/App';

document.querySelector('#app').innerHTML = `
    <div class="input-container">
        <input id="search-input" type="text" placeholder="Type what's on your mind..." autofocus />
        <div id="suggestions" class="suggestions-container"></div>
    </div>
`;

const input = document.getElementById('search-input');
const suggestionsContainer = document.getElementById('suggestions');
const app = document.getElementById('app');

// Focus handling
window.focusInput = function () {
    input.focus();
};

window.addEventListener('focus', () => {
    window.focusInput();
});

// Dynamic Resizing
function updateWindowHeight() {
    // We add some buffer for safe measure, e.g. padding-top of body (10px) + app padding (10px*2) + border?
    // app.offsetHeight includes app's padding and border.
    // body has padding-top: 10px.
    // So total height = app.offsetHeight + 10 (body padding).
    // Let's also add a tiny bit of buffer (e.g. 2px) to prevent scrollbars.
    const height = app.offsetHeight + 12;
    SetWindowHeight(height);
}

// Render suggestions
function renderSuggestions(thoughts, query) {
    suggestionsContainer.innerHTML = '';
    if (!thoughts || thoughts.length === 0) {
        suggestionsContainer.style.display = 'none';
        updateWindowHeight();
        return;
    }

    suggestionsContainer.style.display = 'block';
    const containerDiv = document.createElement('div');
    containerDiv.className = 'suggestions-list-container';

    let lastTime = null;
    let currentGroupUl = null;

    thoughts.forEach(t => {
        const tTime = new Date(t.created_at);

        // Grouping Logic: > 30 mins difference starts a new group
        // If sorting DESC, we compare with previous item time.
        // First item always starts a group.
        let startNewGroup = false;
        if (!lastTime) {
            startNewGroup = true;
        } else {
            const diffMs = Math.abs(lastTime - tTime);
            const diffMins = diffMs / (1000 * 60);
            if (diffMins > 30) {
                startNewGroup = true;
            }
        }

        if (startNewGroup) {
            if (currentGroupUl) {
                containerDiv.appendChild(currentGroupUl);
            }
            // Add a visual separator or header if needed, but for now just a new list or gap
            // Using a simple gap via CSS on the list
            currentGroupUl = document.createElement('ul');
            currentGroupUl.className = 'suggestions-list session-group';
            lastTime = tTime;

            // Optional: Header for the group?
            // User asked for "grouped", maybe a visual indicator.
            // Let's add a small divider or margin in CSS.
        }

        // Update lastTime to current time so we chain them? 
        // Actually for correct "session", if items are within window of EACH OTHER or window of FIRST item?
        // Usually "session" implies interaction flow. If I type entries 1 min apart for an hour, is it one session? Yes.
        // So we should compare with the PREVIOUS item.
        lastTime = tTime;

        const li = document.createElement('li');
        li.className = 'suggestion-item';

        // Highlight logic
        const content = t.content;
        let highlighted = content;
        if (query && query.trim() !== "") {
            const regex = new RegExp(`(${query})`, 'gi');
            highlighted = content.replace(regex, '<span class="highlight">$1</span>');
        }

        li.innerHTML = highlighted;
        currentGroupUl.appendChild(li);
    });

    if (currentGroupUl) {
        containerDiv.appendChild(currentGroupUl);
    }

    suggestionsContainer.appendChild(containerDiv);
    updateWindowHeight();
}

// Hide on Esc, Save on Enter, Search on Input
input.addEventListener('keydown', (e) => {
    if (e.key === 'Escape') {
        if (input.value !== "") {
            input.value = "";
            // If showing recent is enabled, clearing might still show suggestions.
            // But usually user expects clear to hide or reset.
            // Let's call search("") which will handle it (show recent or hide).
            Search("").then(renderSuggestions).catch(console.error);
            e.preventDefault();
        } else {
            Hide();
        }
    } else if (e.key === 'Enter') {
        const val = input.value;
        // Allow empty save? No.
        if (val && val.trim() !== "") {
            Save(val).then(() => {
                input.value = "";
                // After save, update suggestions (might show recent)
                Search("").then(renderSuggestions).catch(console.error);
            }).catch((err) => {
                console.error("Failed to save:", err);
            });
        }
    }
});

input.addEventListener('input', () => {
    const val = input.value;
    // Always search, even if empty (backend handles config)
    Search(val).then((thoughts) => {
        renderSuggestions(thoughts, val);
    }).catch((err) => {
        // If error (e.g. empty search and config off returns nil, wails handles it as null/empty array usually)
        // Actually verify what Search returns when nil. Wails usually returns null.
        console.error("Search failed:", err);
        renderSuggestions([], val);
    });
});

// Hide on Blur
let hideTimeout;
window.addEventListener('blur', () => {
    hideTimeout = setTimeout(() => {
        Hide();
    }, 200); // 200ms grace period
});

window.addEventListener('focus', () => {
    if (hideTimeout) {
        clearTimeout(hideTimeout);
        hideTimeout = null;
    }
    input.focus();
    // Trigger search on focus to show recent entries if configured
    Search(input.value).then((thoughts) => {
        renderSuggestions(thoughts, input.value);
    }).catch(console.error);
});

// Initial focus and resize
input.focus();
setTimeout(() => {
    updateWindowHeight();
    // Also trigger initial search
    Search(input.value).then((thoughts) => {
        renderSuggestions(thoughts, input.value);
    }).catch(console.error);
}, 10);
