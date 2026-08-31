export function sourceBrowserLocation(rawUrl: string | undefined): string {
  if (!rawUrl) return '';
  return rawUrl.startsWith('data:text/html;base64,') ? 'Source-provided HTML document' : rawUrl;
}
