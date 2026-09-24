import { ref, watch } from 'vue';
import { defineStore } from 'pinia';

const key = 'novelreader.import.preferences.v1';

function loadPreferences(): { reviewBeforeAdding: boolean; optimizeEPUBImages: boolean } {
  try {
    const value = JSON.parse(localStorage.getItem(key) || '{}');
    return { reviewBeforeAdding: value.reviewBeforeAdding !== false, optimizeEPUBImages: value.optimizeEPUBImages === true };
  } catch { return { reviewBeforeAdding: true, optimizeEPUBImages: false }; }
}

// Device-local, like reader appearance preferences; not reader-home data.
export const useImportPreferences = defineStore('import-preferences', () => {
  const saved = loadPreferences();
  const reviewBeforeAdding = ref(saved.reviewBeforeAdding);
  const optimizeEPUBImages = ref(saved.optimizeEPUBImages);
  watch([reviewBeforeAdding, optimizeEPUBImages], ([reviewBeforeAdding, optimizeEPUBImages]) => {
    try { localStorage.setItem(key, JSON.stringify({ reviewBeforeAdding, optimizeEPUBImages })); }
    catch { /* Preference persistence is optional when browser storage is unavailable. */ }
  }, { flush: 'sync' });
  return { reviewBeforeAdding, optimizeEPUBImages };
});
