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

let selectedIndex = -1;

function updateSelection(shouldScroll = true) {
    const items = suggestionsContainer.querySelectorAll('.suggestion-item');
    items.forEach((item, index) => {
        if (index === selectedIndex) {
            item.classList.add('selected');
            if (shouldScroll) {
                item.scrollIntoView({ block: 'nearest', behavior: 'smooth' });
            }
        } else {
            item.classList.remove('selected');
        }
    });
}

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
    selectedIndex = -1;
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

        // Content
        const content = t.content;
        let highlighted = content;
        if (query && query.trim() !== "") {
            const regex = new RegExp(`(${query})`, 'gi');
            highlighted = content.replace(regex, '<span class="highlight">$1</span>');
        }

        // Tags
        let tagsHtml = '';
        if (t.tags && t.tags.length > 0) {
            tagsHtml = '<div class="tags-container">';
            t.tags.forEach(tag => {
                tagsHtml += `<span class="tag tag-${tag.toLowerCase()}">${tag}</span>`;
            });
            tagsHtml += '</div>';
        }

        li.innerHTML = `<div class="suggestion-content">${highlighted}</div>${tagsHtml}`;

        // Mouse hover selection
        li.addEventListener('mouseenter', () => {
            selectedIndex = Array.from(containerDiv.querySelectorAll('.suggestion-item')).indexOf(li);
            // Wait, we need the global index across multiple uls?
            // `containerDiv` contains multiple ULs if grouped.
            // `Array.from(suggestionsContainer.querySelectorAll('.suggestion-item'))` is better.
            const allItems = suggestionsContainer.querySelectorAll('.suggestion-item');
            allItems.forEach((item, idx) => {
                if (item === li) {
                    selectedIndex = idx;
                    updateSelection(false); // Don't scroll on hover
                }
            });
        });

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
    const items = suggestionsContainer.querySelectorAll('.suggestion-item');

    if (e.key === 'ArrowDown') {
        e.preventDefault();
        if (items.length > 0) {
            selectedIndex = (selectedIndex + 1) % items.length;
            updateSelection();
        }
    } else if (e.key === 'ArrowUp') {
        e.preventDefault();
        if (items.length > 0) {
            selectedIndex = (selectedIndex - 1 + items.length) % items.length;
            updateSelection();
        }
    } else if (e.key === 'Escape') {
        if (input.value !== "") {
            input.value = "";
            Search("").then(renderSuggestions).catch(console.error);
            e.preventDefault();
        } else {
            Hide();
        }
    } else if (e.key === 'Enter') {
        // Check if item selected
        if (selectedIndex >= 0 && items[selectedIndex]) {
            e.preventDefault();
            const selectedItem = items[selectedIndex];

            // If it's a command
            if (selectedItem.classList.contains('command-item')) {
                const cmdText = selectedItem.querySelector('.command-text').textContent;
                // Set input and Execute
                input.value = cmdText;
                Save(cmdText).then(() => {
                    input.value = "";
                    Search("").then(renderSuggestions).catch(console.error);
                }).catch(console.error);
                return;
            } else {
                // If it's a thought/history item
                // Just autofill for now? Or Copy?
                // Standard history behavior: Fill input.
                // We need the raw content. The HTML might have tags.
                // Extracting text from highlighted content is messy.
                // Better to bind click handler or store data attribute.
                // Let's rely on standard textContent of `.suggestion-content` for now, 
                // but that includes highlight spans. `.textContent` strips tags so it's fine!
                const text = selectedItem.querySelector('.suggestion-content').textContent;
                input.value = text;
                return;
            }
        }

        const val = input.value;
        if (val && val.trim() !== "") {
            Save(val).then(() => {
                input.value = "";
                Search("").then(renderSuggestions).catch(console.error);
            }).catch((err) => {
                console.error("Failed to save:", err);
            });
        }
    }
});

const commands = [
    { command: "/config show-recent true", description: "Enable showing recent entries" },
    { command: "/config show-recent false", description: "Disable showing recent entries" },
    { command: "/config backfill-tags", description: "Backfill tags for existing entries" }
];

function renderCommandSuggestions(inputVal) {
    selectedIndex = -1;
    const matched = commands.filter(c => c.command.startsWith(inputVal));

    suggestionsContainer.innerHTML = '';
    if (!matched || matched.length === 0) {
        suggestionsContainer.style.display = 'none';
        updateWindowHeight();
        return;
    }

    suggestionsContainer.style.display = 'block';
    const containerDiv = document.createElement('div');
    containerDiv.className = 'suggestions-list-container';
    const ul = document.createElement('ul');
    ul.className = 'suggestions-list';

    matched.forEach(c => {
        const li = document.createElement('li');
        li.className = 'suggestion-item command-item';
        // Highlight match
        const content = c.command;
        let highlighted = content;
        if (inputVal && inputVal.trim() !== "") {
            // Escape special chars in inputVal for regex if needed, currently "/" is safe enough usually
            const regex = new RegExp(`(${inputVal})`, 'i'); // just first match?
            highlighted = content.replace(regex, '<span class="highlight">$1</span>');
        }

        li.innerHTML = `
            <div class="suggestion-content">
                <div class="command-text">${highlighted}</div>
                <div class="command-desc">${c.description}</div>
            </div>
        `;

        // Click to autofill/execute
        li.addEventListener('click', () => {
            input.value = c.command;
            input.focus();
            // Optional: Auto-submit?
            // Save(c.command)...
        });

        // Mouse hover selection
        li.addEventListener('mouseenter', () => {
            // Since commands are one list, index is simple match index
            const allItems = suggestionsContainer.querySelectorAll('.suggestion-item');
            allItems.forEach((item, idx) => {
                if (item === li) {
                    selectedIndex = idx;
                    updateSelection(false);
                }
            });
        });

        ul.appendChild(li);
    });

    containerDiv.appendChild(ul);
    suggestionsContainer.appendChild(containerDiv);
    updateWindowHeight();
}

input.addEventListener('input', () => {
    const val = input.value;
    if (val.startsWith('/')) {
        renderCommandSuggestions(val);
        return;
    }

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

    if (input.value.startsWith('/')) {
        renderCommandSuggestions(input.value);
        return;
    }

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
