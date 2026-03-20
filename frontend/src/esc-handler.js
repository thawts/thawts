/**
 * setupEscHandler registers a document-level capture keydown listener for Escape.
 *
 * ESC chain (each press advances one step):
 *   1. A thought is selected  → deselect it and focus the input
 *   2. Input has text         → clear it and focus the input
 *   3. In review mode         → switch to braindump (showCapture)
 *   4. In braindump mode      → hide the window
 *
 * The #inspector-content input handles its own ESC (discard edit) and is
 * skipped here so both handlers don't fire at the same time.
 *
 * @param {object}      opts
 * @param {() => string} opts.getMode          returns current mode ('braindump' | 'review')
 * @param {() => any}    opts.getSelectedId    returns selectedThoughtId (null if none)
 * @param {() => void}   opts.deselectThought  clears selection and re-renders
 * @param {() => string} opts.getInputValue    returns current value of #thought-input
 * @param {() => void}   opts.clearInput       clears #thought-input and fires side-effects
 * @param {() => void}   opts.focusInput       focuses #thought-input
 * @param {() => void}   opts.hideWindow
 * @param {() => void}   opts.showCapture
 * @returns {() => void} teardown function (removes the listener)
 */
export function setupEscHandler({ getMode, getSelectedId, deselectThought, getInputValue, clearInput, focusInput, hideWindow, showCapture }) {
  function handler(e) {
    if (e.key !== 'Escape') return;
    // Let inspector-content handle its own ESC (discard edit).
    if (e.target?.id === 'inspector-content') return;
    e.preventDefault();

    if (getSelectedId() !== null) {
      deselectThought();
      focusInput();
      return;
    }

    if (getInputValue().trim()) {
      // Input has text but isn't focused → just return focus to it (preserving search).
      // If it IS focused, clear it so the next ESC can switch modes.
      if (e.target?.id === 'thought-input') clearInput();
      focusInput();
      return;
    }

    if (getMode() === 'review') {
      showCapture();
    } else {
      hideWindow();
    }
  }
  document.addEventListener('keydown', handler, { capture: true });
  return () => document.removeEventListener('keydown', handler, { capture: true });
}
