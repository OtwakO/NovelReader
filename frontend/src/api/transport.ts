import { isExploreErrorBody } from './errors';

export const API_BASE = '/api';

type AuthenticationLossListener = () => void;
let authenticationLossListener: AuthenticationLossListener | undefined;

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

async function fetchRequest(input: RequestInfo | URL, init?: RequestInit): Promise<Response> {
  try {
    return await fetch(input, init);
  } catch (cause) {
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

async function responseError(response: Response): Promise<never> {
  const error = await response.json().catch(() => ({ error: response.statusText })) as ExploreErrorBody;
  if (response.status === 401) authenticationLossListener?.();
  throw new ApiError(response.status, { ...error, error: error.error || error.message || response.statusText });
}

export async function request<T>(path: string, options?: RequestInit, errorKind: 'general' | 'explore' = 'general'): Promise<T> {
  const response = await fetchRequest(`${API_BASE}${path}`, {
    headers: { 'Content-Type': 'application/json', ...options?.headers },
    ...options,
  });
  if (!response.ok) {
    const error = await response.json().catch(() => ({ error: response.statusText })) as ExploreErrorBody;
    if (errorKind === 'explore' && isExploreErrorBody(error)) {
      if (response.status === 401) authenticationLossListener?.();
      throw new ExploreApiError(error);
    }
    if (response.status === 401) authenticationLossListener?.();
    throw new ApiError(response.status, { ...error, error: error.error || error.message || response.statusText });
  }
  if (response.status === 204) return undefined as T;
  return response.json() as Promise<T>;
}

export async function requestForm<T>(path: string, form: FormData): Promise<T> {
  const response = await fetchRequest(`${API_BASE}${path}`, { method: 'POST', body: form });
  if (!response.ok) return responseError(response);
  return response.json() as Promise<T>;
}

export async function requestBinary(path: string): Promise<Response> {
  const response = await fetchRequest(`${API_BASE}${path}`);
  if (!response.ok) return responseError(response);
  return response;
}

export async function requestUpload<T>(path: string, body: Blob): Promise<T> {
  const response = await fetchRequest(`${API_BASE}${path}`, { method: 'POST', body, headers: { 'Content-Type': 'application/gzip' } });
  if (!response.ok) return responseError(response);
  return response.json() as Promise<T>;
}
