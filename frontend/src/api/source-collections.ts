import { request, requestForm } from './transport';

export type CollectionOrigin = 'upload' | 'url';
export type SyncInterval = 'manual' | 'daily' | 'weekly';

export interface SourceCollection {
  id: string;
  name: string;
  originKind: CollectionOrigin;
  originUrl?: string;
  originFilename?: string;
  syncInterval: SyncInterval;
  sourceCount: number;
  lastAttemptAt?: number;
  lastSuccessAt?: number;
  nextSyncAt?: number;
  lastError?: string;
  createdAt: number;
  updatedAt: number;
}

export interface CollectionChanges {
  added: number;
  updated: number;
  removed: number;
  unchanged: number;
  total: number;
}

export interface CollectionMutation {
  collection: SourceCollection;
  changes: CollectionChanges;
}

export function listSourceCollections() {
  return request<SourceCollection[]>('/source-collections');
}

export function createUploadCollection(name: string, file: File) {
  const form = new FormData();
  form.set('name', name);
  form.set('file', file);
  return requestForm<CollectionMutation>('/source-collections/upload', form);
}

export function createURLCollection(name: string, url: string, syncInterval: SyncInterval) {
  return request<CollectionMutation>('/source-collections/url', {
    method: 'POST',
    body: JSON.stringify({ name, url, syncInterval }),
  });
}

export function updateSourceCollection(id: string, update: { name?: string; syncInterval?: SyncInterval }) {
  return request<SourceCollection>(`/source-collections/${encodeURIComponent(id)}`, {
    method: 'PATCH',
    body: JSON.stringify(update),
  });
}

export function replaceUploadCollection(id: string, file: File) {
  const form = new FormData();
  form.set('file', file);
  return requestForm<CollectionMutation>(`/source-collections/${encodeURIComponent(id)}/replace`, form);
}

export function syncSourceCollection(id: string) {
  return request<CollectionMutation>(`/source-collections/${encodeURIComponent(id)}/sync`, { method: 'POST' });
}

export function deleteSourceCollection(id: string) {
  return request<void>(`/source-collections/${encodeURIComponent(id)}`, { method: 'DELETE' });
}
