import { ref, watch } from 'vue';
import { defineStore } from 'pinia';

const key = 'novelreader.import.preferences.v1';

function loadReviewBeforeAdding(): boolean {
  try { return JSON.parse(localStorage.getItem(key) || '{}').reviewBeforeAdding !== false; }
  catch { return true; }
}

// Device-local, like reader appearance preferences; not reader-home data.
export const useImportPreferences = defineStore('import-preferences', () => {
  const reviewBeforeAdding = ref(loadReviewBeforeAdding());
  watch(reviewBeforeAdding, value => {
    try { localStorage.setItem(key, JSON.stringify({ reviewBeforeAdding: value })); }
    catch { /* Preference persistence is optional when browser storage is unavailable. */ }
  }, { flush: 'sync' });
  return { reviewBeforeAdding };
});
