import { computed, markRaw, onScopeDispose, ref, watch } from 'vue';
import { defineStore } from 'pinia';
import { acceptTXT, acquireTXT, getTXTAdmission, getTXTReceipt, requestTXTAdmission, type TXTAdmission, type TXTReceipt } from '../../api/txt-imports';
import { ApiError, readerRequestSignal } from '../../api/transport';
import { importedTitle } from './import-feedback';
import { useImportPreferences } from './import-preferences';

export interface ImportTransfer {
  key: number; name: string; size?: number; input?: File | string;
  state: 'queued' | 'waiting' | 'transferring' | 'acquired' | 'adding' | 'added' | 'review' | 'attention';
  receiptId?: string; receipt?: TXTReceipt; initialGeneration?: number; libraryId?: string;
  error?: unknown; warnings?: string[]; reviewBeforeAdding?: boolean;
}

const activeStates = ['queued', 'waiting', 'transferring', 'acquired', 'adding'];

function wait(signal: AbortSignal, delay = 5000): Promise<void> {
  signal.throwIfAborted();
  return new Promise((resolve, reject) => {
    const cancel = () => { clearTimeout(timer); reject(signal.reason); };
    const timer = setTimeout(() => { signal.removeEventListener('abort', cancel); resolve(); }, delay);
    signal.addEventListener('abort', cancel, { once: true });
  });
}

export const useImportQueue = defineStore('txt-import-queue', () => {
  const preferences = useImportPreferences();
  const entries = ref<ImportTransfer[]>([]);
  const running = ref(false);
  const paused = ref(false);
  const revision = ref(0);
  const libraryRevision = ref(0); // Refresh a visible shelf only when admission changes it.
  let sequence = 0;
  let controller: AbortController | undefined;
  let publicationController: AbortController | undefined;
  const outstanding = computed(() => entries.value.filter(item => activeStates.includes(item.state)).length);
  const attention = computed(() => entries.value.filter(item => ['review', 'attention'].includes(item.state) || item.error || item.warnings?.length).length);

  const warnBeforeLeaving = (event: BeforeUnloadEvent) => { event.preventDefault(); event.returnValue = ''; };
  watch(outstanding, count => {
    if (count) window.addEventListener('beforeunload', warnBeforeLeaving);
    else window.removeEventListener('beforeunload', warnBeforeLeaving);
  }, { flush: 'sync' });
  onScopeDispose(() => { resetReaderState(); window.removeEventListener('beforeunload', warnBeforeLeaving); });

  function enqueue(inputs: (File | string)[]) {
    const pending: ImportTransfer[] = [];
    const retained = new Set(entries.value.map(item => item.input));
    for (const input of inputs) {
      const name = typeof input === 'string' ? input : input.name;
      // Coalesce repeated clicks/selections only while that same local input is queued.
      if (retained.has(input)) continue;
      retained.add(input);
      pending.push({ key: ++sequence, name, reviewBeforeAdding: preferences.reviewBeforeAdding, size: typeof input === 'string' ? undefined : input.size,
        input: typeof input === 'string' ? input : markRaw(input), state: /\.txt$/i.test(name) ? 'queued' : 'attention',
        error: /\.txt$/i.test(name) ? undefined : new ApiError(400, { code: 'txt_invalid_input' }),
      });
    }
    entries.value.push(...pending);
    void start();
  }

  async function grant(signal: AbortSignal): Promise<TXTAdmission | undefined> {
    let ticket: TXTAdmission | undefined;
    while (!paused.value) {
      try {
        ticket = ticket ? await getTXTAdmission(ticket.id, signal) : await requestTXTAdmission(signal);
        signal.throwIfAborted();
        if (ticket.state === 'granted') return ticket;
        // Another tab may own the active transfer. Never reuse or cancel it.
        if (ticket.state === 'transferring') ticket = undefined;
      } catch (cause) {
        signal.throwIfAborted();
        if (!(cause instanceof ApiError) || !['txt_intake_busy', 'txt_ticket_not_found'].includes(cause.code)) throw cause;
        ticket = undefined;
      }
      await wait(signal);
    }
  }

  async function start() {
    if (running.value || paused.value) return;
    const owner = new AbortController();
    controller = owner;
    const signal = AbortSignal.any([owner.signal, readerRequestSignal()]);
    running.value = true;
    try {
      let entry: ImportTransfer | undefined;
      while (!paused.value && (entry = entries.value.find(item => item.state === 'queued'))) {
        entry.state = 'waiting'; entry.error = undefined;
        let attempted = false;
        try {
          const ticket = await grant(signal);
          signal.throwIfAborted();
          if (!ticket || paused.value) { entry.state = 'queued'; break; }
          if (entry.size !== undefined && entry.size > ticket.maxInputBytes) throw new ApiError(413, { code: 'txt_too_large' });
          entry.receiptId = ticket.id;
          entry.state = 'transferring'; attempted = true;
          const result = await acquireTXT(ticket.id, entry.input!, signal);
          signal.throwIfAborted();
          entry.receiptId = result.receipt.id;
          entry.receipt = result.receipt;
          entry.initialGeneration = result.receipt.analysisVersion;
          entry.warnings = result.warnings;
          entry.state = 'acquired';
          entry.input = undefined; // Release File/Blob ownership at the file boundary.
          revision.value++;
          void publishReadyImports();
        } catch (cause) {
          if (signal.aborted) return;
          entry.error = cause;
          if (cause instanceof ApiError && cause.body.receiptId) entry.receiptId = cause.body.receiptId;
          if (!attempted && !(cause instanceof ApiError && cause.code === 'txt_too_large')) {
            // Admission failed before bytes were sent. Preserve the selection and
            // pause, rather than failing every remaining file during an outage.
            entry.state = 'queued'; paused.value = true;
          } else {
            entry.state = 'attention';
            // Never replay an uncertain acquisition. Its receipt/claim is the recovery path.
            entry.input = undefined;
            if (!(cause instanceof ApiError) || cause.status >= 500) paused.value = true;
          }
        }
      }
    } finally {
      if (controller === owner) { running.value = false; controller = undefined; }
    }
  }

  // Only new exact initial interpretations explicitly queued with review off carry
  // automatic approval. History and explicit review updates never enter this loop.
  async function publishReadyImports() {
    if (publicationController) return;
    const owner = new AbortController();
    publicationController = owner;
    const signal = AbortSignal.any([owner.signal, readerRequestSignal()]);
    let offset = 0;
    try {
      while (true) {
        const pending = entries.value.filter(item => item.state === 'acquired');
        if (!pending.length) return;
        if (offset >= pending.length) offset = 0;
        // Round-robin metadata checks, not one timer/request loop per book. Keep
        // byte transfer independent so server analysis can overlap the next upload.
        const batch = pending.slice(offset, offset + 16);
        offset += batch.length;
        for (const entry of batch) {
          try {
            const receipt = await getTXTReceipt(entry.receiptId!, signal);
            signal.throwIfAborted();
            entry.receipt = receipt;
            if (receipt.state === 'published') { updateReceipt(receipt); continue; }
            if (receipt.analysisVersion !== entry.initialGeneration) throw new ApiError(409, { code: 'txt_state_changed' });
            if (receipt.hasError || entry.warnings?.length || ['needs_review', 'analysis_failed', 'failed', 'removing'].includes(receipt.state)) {
              entry.state = 'attention'; continue;
            }
            if (receipt.state !== 'ready') continue;
            if (entry.reviewBeforeAdding !== false) { entry.state = 'review'; revision.value++; continue; }
            entry.state = 'adding';
            const result = await acceptTXT(receipt.id, receipt.analysisVersion, importedTitle(receipt.originalName), '', signal);
            signal.throwIfAborted();
            updateReceipt({ ...receipt, state: 'published', libraryId: result.libraryId });
          } catch (cause) {
            if (signal.aborted) return;
            // No blind replay of an uncertain accept. Inline review fetches the
            // authoritative status before any further user decision.
            entry.error = cause; entry.state = 'attention';
          }
        }
        if (entries.value.some(item => item.state === 'acquired')) await wait(signal, 1500);
      }
    } catch (cause) { if (!signal.aborted) throw cause; }
    finally { if (publicationController === owner) publicationController = undefined; }
  }

  function updateReceipt(receipt: TXTReceipt) {
    const entry = entries.value.find(item => item.receiptId === receipt.id);
    if (receipt.state === 'published' && receipt.libraryId && entry?.libraryId !== receipt.libraryId) libraryRevision.value++;
    if (entry) {
      entry.receipt = receipt;
      entry.error = undefined;
      if (receipt.state === 'published' && receipt.libraryId) {
        entry.state = 'added'; entry.libraryId = receipt.libraryId;
      }
    }
    revision.value++;
  }

  function resume() { paused.value = false; void start(); }
  function pause() { paused.value = true; } // Finish current bytes; do not start another file.
  function remove(key: number) {
    entries.value = entries.value.filter(item => item.key !== key || ['waiting', 'transferring', 'acquired', 'adding'].includes(item.state));
  }
  function clearFinished() { entries.value = entries.value.filter(item => item.state !== 'added'); }
  function resetReaderState() {
    controller?.abort(); controller = undefined;
    publicationController?.abort(); publicationController = undefined;
    entries.value = []; running.value = false; paused.value = false; revision.value = 0; libraryRevision.value = 0;
  }
  return { entries, running, paused, outstanding, attention, revision, libraryRevision, enqueue, pause, resume, remove, clearFinished, updateReceipt, resetReaderState };
});
