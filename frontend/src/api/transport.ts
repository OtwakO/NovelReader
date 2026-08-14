import { isExploreErrorBody } from './errors';

export const API_BASE = '/api';

type AuthenticationLossListener = () => void;
let authenticationLossListener: AuthenticationLossListener | undefined;

export interface ExploreErrorBody {
  code?: string;
  stage?: string;
  severity?: string;
  retryable?: boolean;
  message?: string;
  error?: string;
  nextPage?: number;
}

export class ExploreApiError extends Error {
  code: string;
  stage: string;
  severity: string;
  retryable: boolean;
  nextPage?: number;

  constructor(body: ExploreErrorBody) {
    super(body.message || 'Explore request failed');
    this.name = 'ExploreApiError';
    this.code = body.code || 'internal_error';
    this.stage = body.stage || 'internal';
    this.severity = body.severity || 'error';
    this.retryable = Boolean(body.retryable);
    this.nextPage = body.nextPage;
  }
}

export class ApiError extends Error {
  status: number;

  constructor(status: number, message: string) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
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
  throw new ApiError(response.status, error.error || error.message || response.statusText);
}

export async function request<T>(path: string, options?: RequestInit, errorKind: 'general' | 'explore' = 'general'): Promise<T> {
  const response = await fetch(`${API_BASE}${path}`, {
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
    throw new ApiError(response.status, error.error || error.message || response.statusText);
  }
  if (response.status === 204) return undefined as T;
  return response.json() as Promise<T>;
}

export async function requestForm<T>(path: string, form: FormData): Promise<T> {
  const response = await fetch(`${API_BASE}${path}`, { method: 'POST', body: form });
  if (!response.ok) return responseError(response);
  return response.json() as Promise<T>;
}
