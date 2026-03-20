import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { setupTabHandler } from './tab-handler.js';

describe('setupTabHandler', () => {
  let showReview, showCapture, mode, teardown;

  beforeEach(() => {
    showReview = vi.fn();
    showCapture = vi.fn();
    mode = 'braindump';
    document.body.innerHTML = '<input id="thought-input">';
    teardown = setupTabHandler(() => mode, showReview, showCapture);
  });

  afterEach(() => {
    teardown();
  });

  it('calls showReview when Tab is pressed in braindump mode', () => {
    const input = document.getElementById('thought-input');
    input.dispatchEvent(new KeyboardEvent('keydown', { key: 'Tab', bubbles: true }));
    expect(showReview).toHaveBeenCalledOnce();
    expect(showCapture).not.toHaveBeenCalled();
  });

  it('calls showCapture when Tab is pressed in review mode', () => {
    mode = 'review';
    const input = document.getElementById('thought-input');
    input.dispatchEvent(new KeyboardEvent('keydown', { key: 'Tab', bubbles: true }));
    expect(showCapture).toHaveBeenCalledOnce();
    expect(showReview).not.toHaveBeenCalled();
  });

  it('does not react to non-Tab keys', () => {
    const input = document.getElementById('thought-input');
    input.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }));
    input.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
    expect(showReview).not.toHaveBeenCalled();
    expect(showCapture).not.toHaveBeenCalled();
  });

  it('does nothing when thought-input is absent', () => {
    teardown();
    document.body.innerHTML = '';
    // should not throw
    teardown = setupTabHandler(() => mode, showReview, showCapture);
  });
});
