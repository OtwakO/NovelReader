import { describe, expect, test } from 'vitest';
import { isAtScrollBoundary, readerTapAction } from './reader-tap-navigation';

const rect = { left: 0, top: 0, width: 300, height: 600 };

describe('reader tap navigation', () => {
  test('maps the five cross zones to reading actions and keeps corners neutral', () => {
    expect(readerTapAction(150, 100, rect)).toBe('scroll-up');
    expect(readerTapAction(50, 300, rect)).toBe('previous');
    expect(readerTapAction(150, 300, rect)).toBe('toggle-controls');
    expect(readerTapAction(250, 300, rect)).toBe('next');
    expect(readerTapAction(150, 500, rect)).toBe('scroll-down');
    expect(readerTapAction(20, 20, rect)).toBe('show-controls');
    expect(readerTapAction(280, 580, rect)).toBe('show-controls');
  });

  test('disables side chapter navigation for coarse touch input', () => {
    expect(readerTapAction(50, 300, rect, false)).toBe('none');
    expect(readerTapAction(250, 300, rect, false)).toBe('none');
    expect(readerTapAction(150, 100, rect, false)).toBe('scroll-up');
    expect(readerTapAction(150, 300, rect, false)).toBe('toggle-controls');
    expect(readerTapAction(150, 500, rect, false)).toBe('scroll-down');
  });

  test('crosses chapters only at the matching scroll boundary', () => {
    expect(isAtScrollBoundary(-1, 0, 1800, 600)).toBe(true);
    expect(isAtScrollBoundary(-1, 50, 1800, 600)).toBe(false);
    expect(isAtScrollBoundary(1, 1200, 1800, 600)).toBe(true);
    expect(isAtScrollBoundary(1, 1100, 1800, 600)).toBe(false);
  });
});
