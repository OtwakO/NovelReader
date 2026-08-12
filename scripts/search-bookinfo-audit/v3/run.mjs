import { execFile } from 'node:child_process';
import fs from 'node:fs';
import { promisify } from 'node:util';

const execFileAsync = promisify(execFile);
const version = 3;
const binary = process.env.CONFORMANCE_BIN ?? '/tmp/novelreader-conformance';
const frozenPath = `/tmp/search-bookinfo-v${version}-frozen.json`;
const outputPath = `/tmp/search-bookinfo-v${version}-initial.json`;
const frozen = JSON.parse(fs.readFileSync(frozenPath, 'utf8'));
const importPath = frozen.exactImport.path;

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
async function run(identity, frozenIndex) {
  const common = ['-sources', importPath, '-indices', String(frozenIndex), '-timeout', '30s'];
  const search = await invoke([...common, '-query', frozen.query]);
  const searchRecord = Array.isArray(search.output) ? search.output[0] : undefined;
  const selectedResult = firstCredible(searchRecord?.extracted);
  const detail = selectedResult ? await invoke([...common, '-detail-result', JSON.stringify(selectedResult)]) : null;
  return { ...identity, frozenIndex, search, selectedResult: selectedResult ?? null, detail };
}

const entries = new Array(frozen.selection.identities.length);
let cursor = 0;
async function worker() {
  while (true) {
    const frozenIndex = cursor++;
    if (frozenIndex >= entries.length) return;
    entries[frozenIndex] = await run(frozen.selection.identities[frozenIndex], frozenIndex);
    fs.writeFileSync(outputPath, `${JSON.stringify({ frozen, entries: entries.filter(Boolean) }, null, 2)}\n`);
    const searchClass = entries[frozenIndex].search.output?.[0]?.classification ?? 'invocation_failure';
    const detailClass = entries[frozenIndex].detail?.output?.classification ?? '-';
    console.log(`${frozenIndex + 1}/${entries.length} raw=${entries[frozenIndex].rawIndex} search=${searchClass} detail=${detailClass}`);
  }
}
await Promise.all(Array.from({ length: 4 }, worker));
fs.writeFileSync(outputPath, `${JSON.stringify({ frozen, entries }, null, 2)}\n`);
