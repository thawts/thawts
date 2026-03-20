/**
 * setupEscHandler registers a document-level keydown listener for Escape.
 *
 * It intentionally skips the #thought-input element — that element has its own
 * keydown handler (onInputKeydown in main.js) which deals with extra cases like
 * dismissing slash suggestions and clearing search text before hiding.
 *
 * @param {() => string} getMode   returns current mode ('braindump' | 'review')
 * @param {() => void}   hideWindow
 * @param {() => void}   showCapture
 * @returns {() => void} teardown function (removes the listener)
 */
export function setupEscHandler(getMode, hideWindow, showCapture) {
  function handler(e) {
    if (e.key !== 'Escape') return;
    if (e.target?.id === 'thought-input') return; // handled by onInputKeydown
    e.preventDefault();
    if (getMode() === 'review') {
      showCapture();
    } else {
      hideWindow();
    }
  }
  document.addEventListener('keydown', handler);
  return () => document.removeEventListener('keydown', handler);
}
