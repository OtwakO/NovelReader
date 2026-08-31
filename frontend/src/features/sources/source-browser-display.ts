export interface SourceBrowserViewport { width: number; height: number; deviceScaleFactor: number }

export function sourceBrowserViewport(availableWidth: number, devicePixelRatio: number): SourceBrowserViewport {
  const width = Math.round(Math.min(430, Math.max(390, availableWidth)));
  return {
    width,
    height: Math.round(width * 1.9),
    deviceScaleFactor: Math.min(2, Math.max(1, devicePixelRatio || 1)),
  };
}

export function sourceBrowserLocation(rawUrl: string | undefined): string {
  if (!rawUrl) return '';
  return rawUrl.startsWith('data:text/html;base64,') ? 'Source-provided HTML document' : rawUrl;
}
