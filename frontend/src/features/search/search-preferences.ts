export type SearchIntensity = 'gentle' | 'balanced' | 'fast' | 'advanced';
export interface SearchPreferences { batchSize: number; intensity: SearchIntensity; advancedConcurrency: number }

const storageKey = 'novelreader.search-preferences';
const defaults: SearchPreferences = { batchSize: 50, intensity: 'balanced', advancedConcurrency: 8 };

function clamp(value: number, minimum: number, maximum: number) { return Math.min(maximum, Math.max(minimum, Math.trunc(value))); }

export function normalizeSearchPreferences(value: Partial<SearchPreferences>): SearchPreferences {
  return {
    batchSize: clamp(Number(value.batchSize) || defaults.batchSize, 1, 500),
    intensity: ['gentle', 'balanced', 'fast', 'advanced'].includes(String(value.intensity)) ? value.intensity as SearchIntensity : defaults.intensity,
    advancedConcurrency: Math.max(1, Math.trunc(Number(value.advancedConcurrency) || defaults.advancedConcurrency)),
  };
}

export function loadSearchPreferences(): SearchPreferences {
  try { return normalizeSearchPreferences(JSON.parse(localStorage.getItem(storageKey) || '{}') as Partial<SearchPreferences>); }
  catch { return { ...defaults }; }
}

export function saveSearchPreferences(preferences: SearchPreferences) { localStorage.setItem(storageKey, JSON.stringify(normalizeSearchPreferences(preferences))); }
export function requestedConcurrency(preferences: SearchPreferences) {
  if (preferences.intensity === 'gentle') return 4;
  if (preferences.intensity === 'fast') return 16;
  if (preferences.intensity === 'advanced') return Math.max(1, preferences.advancedConcurrency);
  return 8;
}
