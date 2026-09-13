import { reactive } from 'vue';
import { readerRequestSignal } from '../../api/transport';

// One view-owned operation at a time. Polls cannot interrupt explicit mutations.
// The owning Options API view cancels this task before unmount or route replacement.
export function createImportTask() {
  const readerSignal = readerRequestSignal();
  let owner = new AbortController();
  const task = reactive({ busy: false, error: undefined as unknown, run, cancel });
  function cancel() { owner.abort(); owner = new AbortController(); task.busy = false; }
  async function run(action: (signal: AbortSignal) => Promise<void>): Promise<boolean> {
    if (task.busy) return false;
    const current = owner;
    const signal = AbortSignal.any([current.signal, readerSignal]);
    task.busy = true; task.error = undefined;
    try { signal.throwIfAborted(); await action(signal); signal.throwIfAborted(); return true; }
    catch (cause) { if (!signal.aborted) task.error = cause; return false; }
    finally { if (owner === current) task.busy = false; }
  }
  return task;
}
