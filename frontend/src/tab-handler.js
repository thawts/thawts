/**
 * setupTabHandler registers a keydown listener on #thought-input for the Tab key.
 * Tab switches between braindump and review mode.
 *
 * @param {() => string} getMode      returns current mode ('braindump' | 'review')
 * @param {() => void}   showReview
 * @param {() => void}   showCapture
 * @returns {() => void} teardown function
 */
export function setupTabHandler(getMode, showReview, showCapture) {
  function handler(e) {
    if (e.key !== 'Tab') return;
    e.preventDefault();
    if (getMode() === 'braindump') {
      showReview();
    } else {
      showCapture();
    }
  }
  const input = document.getElementById('thought-input');
  if (input) input.addEventListener('keydown', handler);
  return () => { if (input) input.removeEventListener('keydown', handler); };
}
