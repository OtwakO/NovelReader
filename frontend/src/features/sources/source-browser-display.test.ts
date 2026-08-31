import { describe, expect, it } from 'vitest';
import { sourceBrowserLocation } from './source-browser-display';

describe('sourceBrowserLocation', () => {
  it('does not render a source HTML data document as visible location text', () => {
    expect(sourceBrowserLocation('data:text/html;base64,PGgxPlNldHRpbmdzPC9oMT4=')).toBe('Source-provided HTML document');
  });

  it('keeps normal browser locations visible', () => {
    expect(sourceBrowserLocation('https://example.test/login')).toBe('https://example.test/login');
  });
});
