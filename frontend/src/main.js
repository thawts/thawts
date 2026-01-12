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
    const ul = document.createElement('ul');
    ul.className = 'suggestions-list';

    thoughts.forEach(t => {
        const li = document.createElement('li');
        li.className = 'suggestion-item';

        // Highlight logic
        const content = t.content;
        const regex = new RegExp(`(${query})`, 'gi');
        const highlighted = content.replace(regex, '<span class="highlight">$1</span>');

        li.innerHTML = highlighted;
        ul.appendChild(li);
    });

    suggestionsContainer.appendChild(ul);
    updateWindowHeight();
}

// Hide on Esc, Save on Enter, Search on Input
input.addEventListener('keydown', (e) => {
    if (e.key === 'Escape') {
        if (input.value !== "") {
            input.value = "";
            suggestionsContainer.style.display = 'none';
            updateWindowHeight();
            e.preventDefault(); // Prevent default behavior just in case
        } else {
            Hide();
        }
    } else if (e.key === 'Enter') {
        const val = input.value;
        if (val && val.trim() !== "") {
            Save(val).then(() => {
                input.value = "";
                suggestionsContainer.style.display = 'none';
                updateWindowHeight();
            }).catch((err) => {
                console.error("Failed to save:", err);
            });
        }
    }
});

input.addEventListener('input', () => {
    const val = input.value;
    if (val && val.trim() !== "") {
        Search(val).then((thoughts) => {
            renderSuggestions(thoughts, val);
        }).catch((err) => {
            console.error("Search failed:", err);
        });
    } else {
        suggestionsContainer.style.display = 'none';
        updateWindowHeight();
    }
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
});

// Initial focus and resize
input.focus();
// Wait a tick for DOM to settle?
setTimeout(updateWindowHeight, 10);
