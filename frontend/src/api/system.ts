import { request } from './transport';

export type WebViewStatusKind = 'not_configured' | 'unavailable' | 'ready';

export interface WebViewStatus {
  status: WebViewStatusKind;
  checkedAt: number;
}

export function getWebViewStatus(): Promise<WebViewStatus> {
  return request<WebViewStatus>('/system/webview-status');
}

export interface ChineseConversionCapability {
  available: boolean;
  engine?: string;
  version?: string;
  presets?: Partial<Record<'simplified' | 'traditional', string>>;
  modes: Array<'simplified' | 'traditional'>;
}

export function getChineseConversionCapability(): Promise<ChineseConversionCapability> {
  return request<ChineseConversionCapability>('/system/chinese-conversion');
}

export async function convertChineseTexts(mode: 'simplified' | 'traditional', texts: string[]): Promise<string[]> {
  const response = await request<{ texts: string[] }>('/system/chinese-conversion', {
    method: 'POST',
    body: JSON.stringify({ mode, texts }),
  });
  return response.texts;
}
