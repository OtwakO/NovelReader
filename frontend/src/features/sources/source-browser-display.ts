export interface SourceBrowserViewport { width: number; height: number; deviceScaleFactor: number }

export function sourceBrowserViewport(availableWidth: number, devicePixelRatio: number): SourceBrowserViewport {
  const width = Math.round(Math.min(430, Math.max(390, availableWidth)));
  return {
    width,
    height: Math.round(width * 1.9),
    deviceScaleFactor: Math.min(3, Math.max(1, devicePixelRatio || 1)),
  };
}

export function isInternalSourceBrowserLocation(rawUrl: string | undefined): boolean {
  return !rawUrl || rawUrl === 'about:blank' || rawUrl.startsWith('data:');
}
