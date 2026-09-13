import { computed, markRaw, onScopeDispose, ref, watch } from 'vue';
import { defineStore } from 'pinia';
import { acquireTXT, getTXTAdmission, requestTXTAdmission, type TXTAdmission } from '../../api/txt-imports';
import { ApiError, readerRequestSignal } from '../../api/transport';

export interface ImportTransfer {
  key: number; name: string; size?: number; input?: File | string;
  state: 'queued' | 'waiting' | 'transferring' | 'acquired' | 'attention';
  receiptId?: string; error?: unknown; warnings?: string[];
}

function wait(signal: AbortSignal): Promise<void> {
  signal.throwIfAborted();
  return new Promise((resolve, reject) => {
    const cancel = () => { clearTimeout(timer); reject(signal.reason); };
    const timer = setTimeout(() => { signal.removeEventListener('abort', cancel); resolve(); }, 5000);
    signal.addEventListener('abort', cancel, { once: true });
  });
}

export const useImportQueue = defineStore('txt-import-queue', () => {
  const entries = ref<ImportTransfer[]>([]);
  const running = ref(false);
  const paused = ref(false);
  const revision = ref(0); // Acquired work tells the visible list to refresh, not poll each receipt.
  let sequence = 0;
  let controller: AbortController | undefined;
  const outstanding = computed(() => entries.value.filter(item => ['queued', 'waiting', 'transferring'].includes(item.state)).length);
  const attention = computed(() => entries.value.filter(item => item.error || item.warnings?.length).length);

  const warnBeforeLeaving = (event: BeforeUnloadEvent) => { event.preventDefault(); event.returnValue = ''; };
  watch(outstanding, count => {
    if (count) window.addEventListener('beforeunload', warnBeforeLeaving);
    else window.removeEventListener('beforeunload', warnBeforeLeaving);
  }, { flush: 'sync' });
  onScopeDispose(() => window.removeEventListener('beforeunload', warnBeforeLeaving));

  function enqueue(inputs: (File | string)[]) {
    const pending: ImportTransfer[] = [];
    const retained = new Set(entries.value.map(item => item.input));
    for (const input of inputs) {
      const name = typeof input === 'string' ? input : input.name;
      // Coalesce repeated clicks/selections only while that same local input is queued.
      if (retained.has(input)) continue;
      retained.add(input);
      pending.push({ key: ++sequence, name, size: typeof input === 'string' ? undefined : input.size,
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
          entry.warnings = result.warnings;
          entry.state = 'acquired';
          entry.input = undefined; // Release File/Blob ownership at the file boundary.
          revision.value++;
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

  function resume() { paused.value = false; void start(); }
  function pause() { paused.value = true; } // Finish current bytes; do not start another file.
  function remove(key: number) {
    entries.value = entries.value.filter(item => item.key !== key || ['waiting', 'transferring'].includes(item.state));
  }
  function clearFinished() { entries.value = entries.value.filter(item => ['queued', 'waiting', 'transferring'].includes(item.state)); }
  function resetReaderState() {
    controller?.abort(); controller = undefined;
    entries.value = []; running.value = false; paused.value = false; revision.value = 0;
  }
  return { entries, running, paused, outstanding, attention, revision, enqueue, pause, resume, remove, clearFinished, resetReaderState };
});
