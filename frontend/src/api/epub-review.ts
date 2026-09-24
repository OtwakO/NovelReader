import { importControl } from './file-imports';
import { ApiError } from './transport';
import { parseStructuredProseDocument } from './structured-prose';

export interface EPUBPreviewTarget { section: number; anchor?: string }
export interface EPUBPreviewEntry { label: string; target?: EPUBPreviewTarget; unavailable: boolean; children: EPUBPreviewEntry[] }
export interface EPUBPreviewNavigation { source: 'publication' | 'sections'; entries: EPUBPreviewEntry[] }

const path = (id: string) => `/imports/epub/receipts/${encodeURIComponent(id)}`;
function object(value: unknown): Record<string, unknown> {
  if (!value || typeof value !== 'object' || Array.isArray(value)) throw new Error('Invalid EPUB preview');
  return value as Record<string, unknown>;
}
function generationResponse(input: unknown, generation: number) {
  const value = object(input);
  if (value.generation !== generation) throw new ApiError(409, { code: 'epub_state_changed' });
  return value;
}

export async function getEPUBPreviewNavigation(id: string, generation: number, totalSections: number, signal: AbortSignal): Promise<EPUBPreviewNavigation> {
  const value = generationResponse(await importControl<unknown>(`${path(id)}/navigation?generation=${generation}`, signal), generation);
  if (!['publication', 'sections'].includes(String(value.source)) || !Array.isArray(value.entries)) throw new Error('Invalid EPUB contents');
  let count = 0;
  const parse = (input: unknown, depth: number): EPUBPreviewEntry => {
    if (++count > 10000 || depth > 128) throw new Error('EPUB contents limit exceeded');
    const entry = object(input);
    if (typeof entry.label !== 'string' || !Array.isArray(entry.children) || (entry.unavailable !== undefined && typeof entry.unavailable !== 'boolean')) throw new Error('Invalid EPUB entry');
    let target: EPUBPreviewTarget | undefined;
    if (entry.target !== undefined) {
      const location = object(entry.target);
      if (typeof location.section !== 'number' || !Number.isSafeInteger(location.section) || location.section < 0 || location.section >= totalSections || (location.anchor !== undefined && typeof location.anchor !== 'string') || entry.unavailable) throw new Error('Invalid EPUB location');
      target = { section: location.section, ...(location.anchor === undefined ? {} : { anchor: location.anchor }) };
    }
    return { label: entry.label, target, unavailable: entry.unavailable === true, children: entry.children.map(child => parse(child, depth + 1)) };
  };
  return { source: value.source as EPUBPreviewNavigation['source'], entries: value.entries.map(entry => parse(entry, 0)) };
}

export async function getEPUBPreviewSection(id: string, generation: number, section: number, signal: AbortSignal) {
  const value = generationResponse(await importControl<unknown>(`${path(id)}/sections/${section}?generation=${generation}`, signal), generation);
  if (value.section !== section || value.version !== 2) throw new Error('Invalid EPUB preview section');
  return parseStructuredProseDocument(value.document);
}
