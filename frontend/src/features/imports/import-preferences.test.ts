import { createPinia, setActivePinia } from 'pinia';
import { afterEach, expect, it } from 'vitest';
import { useImportPreferences } from './import-preferences';

afterEach(() => localStorage.clear());
it('defaults to review and remembers the device preference in a fresh app instance', () => {
  localStorage.clear(); setActivePinia(createPinia());
  expect(useImportPreferences().reviewBeforeAdding).toBe(true);
  expect(useImportPreferences().optimizeEPUBImages).toBe(false);
  useImportPreferences().optimizeEPUBImages = true;
  useImportPreferences().reviewBeforeAdding = false;
  setActivePinia(createPinia());
  expect(useImportPreferences().reviewBeforeAdding).toBe(false);
  expect(useImportPreferences().optimizeEPUBImages).toBe(true);
});
