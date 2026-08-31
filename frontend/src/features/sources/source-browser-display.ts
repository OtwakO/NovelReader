export interface SourceBrowserViewport { width: number; height: number; deviceScaleFactor: number }

export function sourceBrowserViewport(availableWidth: number, availableHeight: number, devicePixelRatio: number): SourceBrowserViewport {
  const width = Math.round(Math.min(430, Math.max(390, availableWidth)));
  return {
    width,
    height: Math.round(Math.min(900, Math.max(width + 80, availableHeight))),
    deviceScaleFactor: Math.min(3, Math.max(1, devicePixelRatio || 1)),
  };
}

export function isInternalSourceBrowserLocation(rawUrl: string | undefined): boolean {
  return !rawUrl || rawUrl === 'about:blank' || rawUrl.startsWith('data:');
}
