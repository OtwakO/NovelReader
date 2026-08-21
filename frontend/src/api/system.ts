import { request } from './transport';

export type WebViewStatusKind = 'not_configured' | 'unavailable' | 'ready';

export interface WebViewStatus {
  status: WebViewStatusKind;
  checkedAt: number;
}

export function getWebViewStatus(): Promise<WebViewStatus> {
  return request<WebViewStatus>('/system/webview-status');
}
