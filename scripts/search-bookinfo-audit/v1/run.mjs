import { execFile } from 'node:child_process';
import fs from 'node:fs';
import { promisify } from 'node:util';

const execFileAsync = promisify(execFile);
const binary = process.env.CONFORMANCE_BIN ?? '/tmp/novelreader-conformance';
const frozenPath = '/tmp/search-bookinfo-v1-frozen.json';
const outputPath = '/tmp/search-bookinfo-v1-initial.json';
const frozen = JSON.parse(fs.readFileSync(frozenPath, 'utf8'));

async function invoke(args, timeout = 45_000) {
  const started = Date.now();
  try {
    const { stdout, stderr } = await execFileAsync(binary, args, { timeout, maxBuffer: 16 << 20 });
    return { durationMs: Date.now() - started, output: JSON.parse(stdout), stderr: stderr.trim() };
  } catch (error) {
    return {
      durationMs: Date.now() - started,
      invocationError: error.killed ? 'audit command timeout' : error.message,
      stdout: String(error.stdout ?? '').slice(0, 2000),
      stderr: String(error.stderr ?? '').slice(0, 2000),
    };
  }
}

function firstCredible(results) {
  return (results ?? []).find((result) => String(result.name ?? '').trim() && String(result.bookUrl ?? '').trim());
}

async function run(identity) {
  const common = ['-sources', frozen.corpus.path, '-indices', String(identity.rawIndex), '-timeout', '30s'];
  const search = await invoke([...common, '-query', frozen.query]);
  const searchRecord = Array.isArray(search.output) ? search.output[0] : undefined;
  const selectedResult = firstCredible(searchRecord?.extracted);
  let detail = null;
  if (selectedResult) {
    detail = await invoke([...common, '-detail-result', JSON.stringify(selectedResult)]);
  }
  return { ...identity, search, selectedResult: selectedResult ?? null, detail };
}

const entries = new Array(frozen.selection.identities.length);
let cursor = 0;
async function worker() {
  while (true) {
    const index = cursor++;
    if (index >= entries.length) return;
    entries[index] = await run(frozen.selection.identities[index]);
    fs.writeFileSync(outputPath, `${JSON.stringify({ frozen, entries: entries.filter(Boolean) }, null, 2)}\n`);
    const searchClass = entries[index].search.output?.[0]?.classification ?? 'invocation_failure';
    const detailClass = entries[index].detail?.output?.classification ?? '-';
    console.log(`${index + 1}/${entries.length} raw=${entries[index].rawIndex} search=${searchClass} detail=${detailClass}`);
  }
}
await Promise.all(Array.from({ length: 4 }, worker));
fs.writeFileSync(outputPath, `${JSON.stringify({ frozen, entries }, null, 2)}\n`);
