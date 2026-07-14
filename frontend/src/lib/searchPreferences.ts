// Single persisted source for user-controlled search coverage and intensity.
export type SearchIntensity = 'gentle' | 'balanced' | 'fast' | 'advanced';

export interface SearchPreferences {
  batchSize: number;
  intensity: SearchIntensity;
  advancedConcurrency: number;
}

const STORAGE_KEY = 'nr_search_preferences';
const defaults: SearchPreferences = { batchSize: 50, intensity: 'balanced', advancedConcurrency: 8 };

export function loadSearchPreferences(): SearchPreferences {
  try {
    const saved = JSON.parse(localStorage.getItem(STORAGE_KEY) || '{}');
    return {
      batchSize: clamp(Number(saved.batchSize) || defaults.batchSize, 1, 500),
      intensity: ['gentle', 'balanced', 'fast', 'advanced'].includes(saved.intensity)
        ? saved.intensity : defaults.intensity,
      advancedConcurrency: Math.max(1, Number(saved.advancedConcurrency) || defaults.advancedConcurrency),
    };
  } catch {
    return { ...defaults };
  }
}

export function saveSearchPreferences(preferences: SearchPreferences) {
  localStorage.setItem(STORAGE_KEY, JSON.stringify(preferences));
}

export function requestedConcurrency(preferences: SearchPreferences): number {
  if (preferences.intensity === 'gentle') return 4;
  if (preferences.intensity === 'fast') return 16;
  if (preferences.intensity === 'advanced') return Math.max(1, preferences.advancedConcurrency);
  return 8;
}

const clamp = (value: number, minimum: number, maximum: number) =>
  Math.min(maximum, Math.max(minimum, Math.trunc(value)));
