import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { setupEscHandler } from './esc-handler.js';

describe('setupEscHandler', () => {
  let mode, selectedId, inputValue;
  let deselectThought, clearInput, focusInput, hideWindow, showCapture;
  let teardown;

  function makeOpts() {
    return {
      getMode:          () => mode,
      getSelectedId:    () => selectedId,
      deselectThought,
      getInputValue:    () => inputValue,
      clearInput,
      focusInput,
      hideWindow,
      showCapture,
    };
  }

  beforeEach(() => {
    mode = 'braindump';
    selectedId = null;
    inputValue = '';
    deselectThought = vi.fn();
    clearInput = vi.fn(() => { inputValue = ''; });
    focusInput = vi.fn();
    hideWindow = vi.fn();
    showCapture = vi.fn();
    document.body.innerHTML = '<input id="thought-input"><input id="inspector-content">';
    teardown = setupEscHandler(makeOpts());
  });

  afterEach(() => {
    teardown();
  });

  function pressEsc(target = document) {
    target.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
  }

  it('step 1: deselects thought and focuses input when a thought is selected', () => {
    selectedId = 'abc';
    pressEsc();
    expect(deselectThought).toHaveBeenCalledOnce();
    expect(focusInput).toHaveBeenCalledOnce();
    expect(clearInput).not.toHaveBeenCalled();
    expect(showCapture).not.toHaveBeenCalled();
    expect(hideWindow).not.toHaveBeenCalled();
  });

  it('step 2: clears input and focuses when input is focused and has text (no selection)', () => {
    inputValue = 'hello';
    const input = document.getElementById('thought-input');
    input.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
    expect(clearInput).toHaveBeenCalledOnce();
    expect(focusInput).toHaveBeenCalledOnce();
    expect(deselectThought).not.toHaveBeenCalled();
    expect(showCapture).not.toHaveBeenCalled();
    expect(hideWindow).not.toHaveBeenCalled();
  });

  it('step 2: focuses input without clearing when input has text but is not focused (e.g. after delete)', () => {
    inputValue = 'hello';
    mode = 'review';
    pressEsc(); // fired from document, not from #thought-input
    expect(clearInput).not.toHaveBeenCalled();
    expect(focusInput).toHaveBeenCalledOnce();
    expect(showCapture).not.toHaveBeenCalled();
    expect(hideWindow).not.toHaveBeenCalled();
  });

  it('step 3: calls showCapture when in review mode with empty input and no selection', () => {
    mode = 'review';
    pressEsc();
    expect(showCapture).toHaveBeenCalledOnce();
    expect(hideWindow).not.toHaveBeenCalled();
    expect(deselectThought).not.toHaveBeenCalled();
    expect(clearInput).not.toHaveBeenCalled();
  });

  it('step 4: calls hideWindow when in braindump mode with empty input and no selection', () => {
    pressEsc();
    expect(hideWindow).toHaveBeenCalledOnce();
    expect(showCapture).not.toHaveBeenCalled();
  });

  it('does not react to non-Escape keys', () => {
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }));
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Tab', bubbles: true }));
    expect(hideWindow).not.toHaveBeenCalled();
    expect(showCapture).not.toHaveBeenCalled();
    expect(deselectThought).not.toHaveBeenCalled();
  });

  it('skips when #inspector-content is the event target', () => {
    const inspector = document.getElementById('inspector-content');
    inspector.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
    expect(hideWindow).not.toHaveBeenCalled();
    expect(showCapture).not.toHaveBeenCalled();
    expect(deselectThought).not.toHaveBeenCalled();
  });

  it('step 1 takes priority over step 2 (selected + text)', () => {
    selectedId = 'abc';
    inputValue = 'hello';
    pressEsc();
    expect(deselectThought).toHaveBeenCalledOnce();
    expect(clearInput).not.toHaveBeenCalled();
  });
});
