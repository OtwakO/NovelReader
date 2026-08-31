import { describe, expect, it } from 'vitest';
import { isInternalSourceBrowserLocation, sourceBrowserViewport } from './source-browser-display';

describe('sourceBrowserViewport', () => {
  it('uses a portrait mobile viewport on desktop displays', () => {
    expect(sourceBrowserViewport(1180, 2)).toEqual({ width: 430, height: 817, deviceScaleFactor: 2 });
  });

  it('keeps narrow clients portrait and bounds density', () => {
    expect(sourceBrowserViewport(360, 4)).toEqual({ width: 390, height: 741, deviceScaleFactor: 3 });
  });
});

describe('isInternalSourceBrowserLocation', () => {
  it('hides internal blank and data document locations', () => {
    expect(isInternalSourceBrowserLocation('about:blank')).toBe(true);
    expect(isInternalSourceBrowserLocation('data:text/html;base64,PGgxPlNldHRpbmdzPC9oMT4=')).toBe(true);
  });

  it('keeps normal browser locations visible', () => {
    expect(isInternalSourceBrowserLocation('https://example.test/login')).toBe(false);
  });
});
