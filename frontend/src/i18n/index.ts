import { createI18n } from 'vue-i18n';
import { detectInitialLocale, fallbackLocale, isSupportedLocale, persistLocale, supportedLocales, type SupportedLocale } from './locales';

type MessageModule = { default: Record<string, unknown> };
const messageModules = import.meta.glob<MessageModule>('./messages/*/*.ts');
const loadedLocales = new Set<SupportedLocale>();

export const i18n = createI18n({
  legacy: false,
  globalInjection: true,
  locale: fallbackLocale,
  fallbackLocale,
  messages: {},
});

async function loadMessages(locale: SupportedLocale) {
  if (loadedLocales.has(locale)) return;
  const prefix = `./messages/${locale}/`;
  const entries = await Promise.all(
    Object.entries(messageModules)
      .filter(([path]) => path.startsWith(prefix))
      .map(async ([path, load]) => {
        const namespace = path.slice(prefix.length, -3);
        return [namespace, (await load()).default] as const;
      }),
  );
  const messages = Object.fromEntries(entries);
  i18n.global.setLocaleMessage(locale, messages);
  loadedLocales.add(locale);
}

export async function setLocale(locale: SupportedLocale) {
  await loadMessages(locale);
  i18n.global.locale.value = locale;
  persistLocale(locale);
  const metadata = supportedLocales.find((item) => item.code === locale);
  document.documentElement.lang = metadata?.htmlLang ?? locale;
}

export async function initializeI18n() {
  await setLocale(detectInitialLocale());
}

export function localeFromValue(value: string): SupportedLocale {
  return isSupportedLocale(value) ? value : fallbackLocale;
}
