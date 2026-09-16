import { createPinia, setActivePinia } from 'pinia';
import { afterEach, expect, it } from 'vitest';
import { useImportPreferences } from './import-preferences';

afterEach(() => localStorage.clear());
it('defaults to review and remembers the device preference in a fresh app instance', () => {
  localStorage.clear(); setActivePinia(createPinia());
  expect(useImportPreferences().reviewBeforeAdding).toBe(true);
  useImportPreferences().reviewBeforeAdding = false;
  setActivePinia(createPinia());
  expect(useImportPreferences().reviewBeforeAdding).toBe(false);
});
