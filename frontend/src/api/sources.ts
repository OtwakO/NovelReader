import { request } from './transport';

export interface SourceInteractionControl {
  id: string;
  type: 'info' | 'text' | 'password' | 'input' | 'button' | 'toggle' | 'select' | 'unsupported';
  label: string;
  value?: string;
  actionId?: string;
  options?: string[];
  unsupported?: string;
}

export interface SourceInteractionView {
  sourceId: string;
  title: string;
  revision: string;
  controls: SourceInteractionControl[];
}

export interface SourceInteractionEffect {
  type: 'notice' | 'refresh_explore' | 'open_external' | 'search' | 'browser_required';
  message?: string;
  url?: string;
  title?: string;
  await?: boolean;
  browserRequestId?: string;
}

export interface SourceInteractionActionResult {
  view: SourceInteractionView;
  effects: SourceInteractionEffect[];
}

export interface SourceBrowserFrame {
  sessionId: string;
  image: string;
  mediaType: string;
  width: number;
  height: number;
  url: string;
  title: string;
}

export interface SourceBrowserInput {
  type: 'click' | 'type' | 'key' | 'scroll';
  x?: number;
  y?: number;
  text?: string;
  key?: string;
}

export type SourceInteractionResetScope = 'login' | 'settings' | 'all';

export interface RuntimeCookieMetadata {
  scope: string;
  names: string[];
}

export interface RuntimeCookie {
  scope: string;
  header: string;
}

export interface BookSource {
  sourceId?: string; bookSourceUrl: string; bookSourceName: string; bookSourceGroup?: string; bookSourceType?: number; enabled: boolean; enabledExplore: boolean; collectionId?: string;
  searchUrl?: string; ruleSearch?: unknown; ruleBookInfo?: unknown; ruleToc?: unknown; ruleContent?: unknown; header?: string;
  [key: string]: unknown;
}

export function listSources() { return request<BookSource[]>('/sources'); }
export function importSources(data: string) { return request<{ imported: number; total: number }>('/sources', { method: 'POST', body: data }); }
export function updateSource(sourceId: string, source: BookSource) { return request<BookSource>(`/sources?id=${encodeURIComponent(sourceId)}`, { method: 'PUT', body: JSON.stringify(source) }); }
export function deleteSource(sourceId: string) { return request<{ status: string }>(`/sources?id=${encodeURIComponent(sourceId)}`, { method: 'DELETE' }); }
export function getSourceInteraction(sourceId: string) { return request<SourceInteractionView>(`/sources/${encodeURIComponent(sourceId)}/interaction`); }
export function runSourceInteractionAction(sourceId: string, revision: string, actionId: string, values: Record<string, string>, isLongClick = false) {
  return request<SourceInteractionActionResult>(`/sources/${encodeURIComponent(sourceId)}/interaction/actions`, { method: 'POST', body: JSON.stringify({ revision, actionId, values, isLongClick }) });
}
export function resetSourceInteraction(sourceId: string, scope: SourceInteractionResetScope) {
  const suffix = scope === 'all' ? '' : `/${scope}`;
  return request<SourceInteractionView>(`/sources/${encodeURIComponent(sourceId)}/interaction${suffix}`, { method: 'DELETE' });
}
export function getSourceRuntimeCookies(sourceId: string) {
  return request<{ cookies: RuntimeCookieMetadata[] }>(`/sources/${encodeURIComponent(sourceId)}/interaction/runtime-cookies`);
}
export function revealSourceRuntimeCookies(sourceId: string, currentPassword: string) {
  return request<{ cookies: RuntimeCookie[] }>(`/sources/${encodeURIComponent(sourceId)}/interaction/runtime-cookies/reveal`, { method: 'POST', body: JSON.stringify({ currentPassword }) });
}
export function replaceSourceRuntimeCookies(sourceId: string, currentPassword: string, cookies: RuntimeCookie[]) {
  return request<{ cookies: RuntimeCookieMetadata[] }>(`/sources/${encodeURIComponent(sourceId)}/interaction/runtime-cookies`, { method: 'PUT', body: JSON.stringify({ currentPassword, cookies }) });
}
export function startSourceBrowser(sourceId: string, browserRequestId: string, width: number, height: number, deviceScaleFactor: number) {
  return request<SourceBrowserFrame>(`/sources/${encodeURIComponent(sourceId)}/interaction/browser`, { method: 'POST', body: JSON.stringify({ browserRequestId, width, height, deviceScaleFactor }) });
}
export function getSourceBrowserFrame(sourceId: string, sessionId: string) {
  return request<SourceBrowserFrame>(`/sources/${encodeURIComponent(sourceId)}/interaction/browser/${encodeURIComponent(sessionId)}`);
}
export function sendSourceBrowserInput(sourceId: string, sessionId: string, input: SourceBrowserInput) {
  return request<SourceBrowserFrame>(`/sources/${encodeURIComponent(sourceId)}/interaction/browser/${encodeURIComponent(sessionId)}/input`, { method: 'POST', body: JSON.stringify(input) });
}
export function closeSourceBrowser(sourceId: string, sessionId: string, save: boolean) {
  return request<{ closed: boolean; resumed?: SourceInteractionActionResult }>(`/sources/${encodeURIComponent(sourceId)}/interaction/browser/${encodeURIComponent(sessionId)}?save=${save}`, { method: 'DELETE' });
}
