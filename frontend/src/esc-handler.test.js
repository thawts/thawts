import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { setupEscHandler } from './esc-handler.js';

describe('setupEscHandler', () => {
  let hideWindow, showCapture, mode, teardown;

  beforeEach(() => {
    hideWindow = vi.fn();
    showCapture = vi.fn();
    mode = 'braindump';
    document.body.innerHTML = '<input id="thought-input">';
    teardown = setupEscHandler(() => mode, hideWindow, showCapture);
  });

  afterEach(() => {
    teardown();
  });

  it('calls HideWindow when Escape is pressed in capture mode', () => {
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
    expect(hideWindow).toHaveBeenCalledOnce();
    expect(showCapture).not.toHaveBeenCalled();
  });

  it('calls ShowCapture when Escape is pressed in review mode', () => {
    mode = 'review';
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
    expect(showCapture).toHaveBeenCalledOnce();
    expect(hideWindow).not.toHaveBeenCalled();
  });

  it('does not react to non-Escape keys', () => {
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }));
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Tab', bubbles: true }));
    expect(hideWindow).not.toHaveBeenCalled();
    expect(showCapture).not.toHaveBeenCalled();
  });

  it('does not fire when #thought-input is the event target (handled by onInputKeydown)', () => {
    const input = document.getElementById('thought-input');
    input.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
    expect(hideWindow).not.toHaveBeenCalled();
    expect(showCapture).not.toHaveBeenCalled();
  });
});
