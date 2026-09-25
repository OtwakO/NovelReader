import { API_BASE } from './transport';

export interface ContentResourceReference { href: string; mediaType?: string; unavailable?: true }

export function parseContentResource(value: unknown): ContentResourceReference {
  if (!value || typeof value !== 'object') throw new Error('Invalid content resource');
  const resource = value as Record<string, unknown>;
  if (resource.unavailable === true && resource.href === '') return { href: '', unavailable: true };
  if (resource.unavailable !== undefined && resource.unavailable !== false) throw new Error('Invalid content resource state');
  if (typeof resource.href !== 'string' || !resource.href.startsWith(`${API_BASE}/`)) throw new Error('Invalid content resource');
  const url = new URL(resource.href, 'https://reader.invalid');
  if (url.origin !== 'https://reader.invalid' || !url.pathname.startsWith(`${API_BASE}/`)) throw new Error('Invalid content resource');
  return { href: resource.href, ...(typeof resource.mediaType === 'string' ? { mediaType: resource.mediaType } : {}) };
}
