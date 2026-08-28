import type { BookSource } from '../../api/sources';

export interface SourceImportPreview { source: BookSource; selected: boolean; key: string }

function sourceValues(value: unknown): unknown[] {
  if (Array.isArray(value)) return value;
  if (value && typeof value === 'object') {
    const record = value as Record<string, unknown>;
    if (Array.isArray(record.sources)) return record.sources;
    if (Array.isArray(record.bookSources)) return record.bookSources;
    return [value];
  }
  return [];
}

export function parseSourceImport(text: string): SourceImportPreview[] {
  const parsed = JSON.parse(text) as unknown;
  const previews: SourceImportPreview[] = [];
  for (const value of sourceValues(parsed)) {
    if (!value || typeof value !== 'object') continue;
    const source = value as BookSource;
    const url = typeof source.bookSourceUrl === 'string' ? source.bookSourceUrl.trim() : '';
    const name = typeof source.bookSourceName === 'string' ? source.bookSourceName.trim() : '';
    if (!url || !name) continue;
    previews.push({ source: { ...source, bookSourceUrl: url, bookSourceName: name }, selected: false, key: `${url}\u0000${previews.length}` });
  }
  if (!previews.length) throw new Error('No valid BookSource entries were found');
  return previews;
}

export function selectedImportJSON(previews: SourceImportPreview[]): string {
  return JSON.stringify(previews.filter(item => item.selected).map(item => item.source));
}

function hasSourceRule(value: unknown): boolean {
  if (typeof value === 'string') return value.trim().length > 0;
  return value !== null && typeof value === 'object';
}

export function isSearchEligible(source: BookSource): boolean {
  return source.enabled && (source.bookSourceType ?? 0) === 0 && hasSourceRule(source.searchUrl) && hasSourceRule(source.ruleSearch);
}

function sourceDefinitionText(source: BookSource): string {
  try {
    return JSON.stringify(source);
  } catch {
    return "";
  }
}

export function isJavaScriptSource(source: BookSource): boolean {
  return Boolean(source.jsLib) || /<js>|@js:|java\.[a-z]/i.test(sourceDefinitionText(source));
}

export function isWebViewSource(source: BookSource): boolean {
  return /\\?["']webView\\?["']\s*:\s*true/i.test(sourceDefinitionText(source));
}

export function sourceCapabilities(source: BookSource): string[] {
  const values: string[] = [];
  if (source.searchUrl) values.push("search");
  if (source.enabledExplore && source.exploreUrl) values.push("explore");
  if (source.header) values.push("headers");
  if (isJavaScriptSource(source)) values.push("javascript");
  if (isWebViewSource(source)) values.push("webview");
  return values;
}
