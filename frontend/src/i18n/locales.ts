export const supportedLocales = [
  { code: 'en', label: 'English', htmlLang: 'en' },
  { code: 'zh-CN', label: '简体中文', htmlLang: 'zh-CN' },
  { code: 'zh-TW', label: '繁體中文', htmlLang: 'zh-TW' },
] as const;

export type SupportedLocale = typeof supportedLocales[number]['code'];

export const fallbackLocale: SupportedLocale = 'en';
const storageKey = 'novelreader.ui-locale';

export function isSupportedLocale(value: string | null | undefined): value is SupportedLocale {
  return supportedLocales.some((locale) => locale.code === value);
}

export function detectInitialLocale(): SupportedLocale {
  const stored = localStorage.getItem(storageKey);
  if (isSupportedLocale(stored)) return stored;

  for (const candidate of navigator.languages) {
    if (isSupportedLocale(candidate)) return candidate;
    const normalized = candidate.toLowerCase();
    if (normalized === 'zh-tw' || normalized === 'zh-hk' || normalized === 'zh-hant') return 'zh-TW';
    if (normalized === 'zh' || normalized.startsWith('zh-cn') || normalized.startsWith('zh-sg') || normalized.includes('hans')) return 'zh-CN';
    if (normalized.startsWith('en')) return 'en';
  }
  return fallbackLocale;
}

export function persistLocale(locale: SupportedLocale) {
  localStorage.setItem(storageKey, locale);
}
