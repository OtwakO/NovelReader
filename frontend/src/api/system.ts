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

export function parseChineseConversionCapability(input: unknown): ChineseConversionCapability {
  if (!input || typeof input !== 'object') throw new Error('Invalid Chinese conversion capability');
  const value = input as Record<string, unknown>;
  if (typeof value.available !== 'boolean' || !Array.isArray(value.modes) || !value.modes.every(mode => mode === 'simplified' || mode === 'traditional') ||
    [value.engine, value.version].some(field => field !== undefined && typeof field !== 'string') ||
    (value.presets !== undefined && (!value.presets || typeof value.presets !== 'object' || Array.isArray(value.presets) || !Object.values(value.presets).every(preset => typeof preset === 'string')))) throw new Error('Invalid Chinese conversion capability');
  return value as unknown as ChineseConversionCapability;
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
