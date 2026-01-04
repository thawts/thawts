import './style.css';
import { Hide, Save } from '../wailsjs/go/main/App';

document.querySelector('#app').innerHTML = `
    <div class="input-container">
        <input id="search-input" type="text" placeholder="Type a command..." autofocus />
    </div>
`;

const input = document.getElementById('search-input');

// Focus handling
window.addEventListener('focus', () => {
    input.focus();
});

// Hide on Esc, Save on Enter
input.addEventListener('keydown', (e) => {
    if (e.key === 'Escape') {
        Hide();
    } else if (e.key === 'Enter') {
        const val = input.value;
        if (val && val.trim() !== "") {
            Save(val).then(() => {
                input.value = "";
            }).catch((err) => {
                console.error("Failed to save:", err);
            });
        }
    }
});

// Hide on Blur
// Note: In Wails, blurring the window might need backend coordination, 
// but 'blur' event on window often works for webview focus loss.
window.addEventListener('blur', () => {
    Hide();
});

// Initial focus
input.focus();
