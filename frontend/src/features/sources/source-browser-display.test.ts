import { describe, expect, it } from 'vitest';
import { sourceBrowserLocation, sourceBrowserViewport } from './source-browser-display';

describe('sourceBrowserViewport', () => {
  it('uses a portrait mobile viewport on desktop displays', () => {
    expect(sourceBrowserViewport(1180, 2)).toEqual({ width: 430, height: 817, deviceScaleFactor: 2 });
  });

  it('keeps narrow clients portrait and bounds density', () => {
    expect(sourceBrowserViewport(360, 3)).toEqual({ width: 390, height: 741, deviceScaleFactor: 2 });
  });
});

describe('sourceBrowserLocation', () => {
  it('does not render a source HTML data document as visible location text', () => {
    expect(sourceBrowserLocation('data:text/html;base64,PGgxPlNldHRpbmdzPC9oMT4=')).toBe('Source-provided HTML document');
  });

  it('keeps normal browser locations visible', () => {
    expect(sourceBrowserLocation('https://example.test/login')).toBe('https://example.test/login');
  });
});
