import { describe, expect, it } from 'vitest';
import { readerKeyboardAction } from './reader-keyboard';

const event = (key: string, target: HTMLElement | null = null, extra = {}) => ({ key, target, defaultPrevented: false, ctrlKey: false, metaKey: false, altKey: false, shiftKey: false, ...extra });

describe('reader keyboard controls', () => {
  it('maps conservative reading shortcuts', () => {
    expect(readerKeyboardAction(event('ArrowLeft'), false)).toBe('previous');
    expect(readerKeyboardAction(event('ArrowRight'), false)).toBe('next');
    expect(readerKeyboardAction(event('PageUp'), false)).toBe('page-up');
    expect(readerKeyboardAction(event(' '), false)).toBe('page-down');
    expect(readerKeyboardAction(event('Home'), false)).toBe('top');
    expect(readerKeyboardAction(event('Escape'), true)).toBe('escape');
  });

  it('ignores navigation while editing, selecting text, or using an overlay', () => {
    const input = document.createElement('input');
    expect(readerKeyboardAction(event('ArrowRight', input), false)).toBe('none');
    expect(readerKeyboardAction(event('ArrowRight'), false, 'selected prose')).toBe('none');
    expect(readerKeyboardAction(event('ArrowRight'), true)).toBe('none');
  });
});
