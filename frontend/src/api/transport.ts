import { isExploreErrorBody } from './errors';

export const API_BASE = '/api';

type AuthenticationLossListener = () => void;
let authenticationLossListener: AuthenticationLossListener | undefined;
let readerRequests = new AbortController();

export function readerRequestSignal(): AbortSignal { return readerRequests.signal; }

export function resetReaderRequests(): void {
  readerRequests.abort();
  readerRequests = new AbortController();
}

export interface ApiErrorBody {
  code?: string;
  stage?: string;
  classification?: string;
  severity?: string;
  retryable?: boolean;
  message?: string;
  error?: string;
  nextPage?: number;
  workflow?: string;
  attempts?: unknown;
}

export type ExploreErrorBody = ApiErrorBody;

export class ExploreApiError extends Error {
  code: string;
  stage: string;
  classification: string;
  severity: string;
  retryable: boolean;
  nextPage?: number;

  constructor(body: ExploreErrorBody) {
    super(body.message || 'Explore request failed');
    this.name = 'ExploreApiError';
    this.code = body.code || 'internal_error';
    this.stage = body.stage || 'internal';
    this.classification = body.classification || 'unknown';
    this.severity = body.severity || 'error';
    this.retryable = Boolean(body.retryable);
    this.nextPage = body.nextPage;
  }
}

export class NetworkError extends Error {
  constructor(cause?: unknown) {
    super('Network request failed', { cause });
    this.name = 'NetworkError';
  }
}

async function fetchRequest(input: RequestInfo | URL, init?: RequestInit) {
  const signal = init?.signal ? AbortSignal.any([readerRequests.signal, init.signal]) : readerRequests.signal;
  try {
    const response = await fetch(input, { ...init, signal });
    signal.throwIfAborted();
    return { response, signal };
  } catch (cause) {
    signal.throwIfAborted();
    throw new NetworkError(cause);
  }
}

export class ApiError extends Error {
  status: number;
  code: string;
  body: ApiErrorBody;

  constructor(status: number, body: ApiErrorBody | string) {
    const details = typeof body === 'string' ? { error: body } : body;
    super(details.error || details.message || 'Request failed');
    this.name = 'ApiError';
    this.status = status;
    this.code = details.code || 'request_failed';
    this.body = details;
  }
}

export function onAuthenticationLoss(listener?: AuthenticationLossListener) {
  authenticationLossListener = listener;
  return () => {
    if (authenticationLossListener === listener) authenticationLossListener = undefined;
  };
}

async function readJSON<T>(response: Response, signal: AbortSignal, errorKind: 'general' | 'explore' = 'general'): Promise<T> {
  signal.throwIfAborted();
  if (!response.ok) {
    const error = await response.json().catch(() => ({ error: response.statusText })) as ExploreErrorBody;
    signal.throwIfAborted();
    if (response.status === 401) authenticationLossListener?.();
    if (errorKind === 'explore' && isExploreErrorBody(error)) throw new ExploreApiError(error);
    throw new ApiError(response.status, { ...error, error: error.error || error.message || response.statusText });
  }
  if (response.status === 204) return undefined as T;
  const value = await response.json() as T;
  signal.throwIfAborted();
  return value;
}

export async function request<T>(path: string, options?: RequestInit, errorKind: 'general' | 'explore' = 'general'): Promise<T> {
  const { response, signal } = await fetchRequest(`${API_BASE}${path}`, {
    headers: { 'Content-Type': 'application/json', ...options?.headers },
    ...options,
  });
  return readJSON<T>(response, signal, errorKind);
}

export async function requestForm<T>(path: string, form: FormData): Promise<T> {
  const { response, signal } = await fetchRequest(`${API_BASE}${path}`, { method: 'POST', body: form });
  return readJSON<T>(response, signal);
}

export async function requestBinary(path: string): Promise<Response> {
  const { response, signal } = await fetchRequest(`${API_BASE}${path}`);
  if (!response.ok) await readJSON<never>(response, signal);
  signal.throwIfAborted();
  return response;
}

export async function requestUpload<T>(path: string, body: Blob): Promise<T> {
  const { response, signal } = await fetchRequest(`${API_BASE}${path}`, { method: 'POST', body, headers: { 'Content-Type': 'application/gzip' } });
  return readJSON<T>(response, signal);
}
