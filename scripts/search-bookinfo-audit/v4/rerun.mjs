import { execFile } from 'node:child_process';
import fs from 'node:fs';
import { promisify } from 'node:util';

const execFileAsync = promisify(execFile);
const version = 4;
const binary = process.env.CONFORMANCE_BIN ?? '/tmp/novelreader-conformance';
const initial = JSON.parse(fs.readFileSync(`/tmp/search-bookinfo-v${version}-initial.json`, 'utf8'));
const importPath = initial.frozen.exactImport.path;
const outputPath = `/tmp/search-bookinfo-v${version}-rerun.json`;
function isPass(entry) {
  return entry.search.output?.[0]?.classification === 'success' && entry.detail?.output?.classification === 'success';
}
async function invoke(args, timeout = 60_000) {
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
async function rerun(entry) {
  const common = ['-sources', importPath, '-indices', String(entry.frozenIndex), '-timeout', '45s'];
  const search = await invoke([...common, '-query', initial.frozen.query]);
  const selectedResult = firstCredible(search.output?.[0]?.extracted);
  const detail = selectedResult ? await invoke([...common, '-detail-result', JSON.stringify(selectedResult)]) : null;
  return {
    rawIndex: entry.rawIndex,
    bookSourceUrl: entry.bookSourceUrl,
    frozenIndex: entry.frozenIndex,
    search,
    selectedResult: selectedResult ?? null,
    detail,
  };
}

const candidates = initial.entries.filter((entry) => !isPass(entry));
const entries = [];
for (const [index, entry] of candidates.entries()) {
  const result = await rerun(entry);
  entries.push(result);
  fs.writeFileSync(outputPath, `${JSON.stringify({ frozen: initial.frozen, entries }, null, 2)}\n`);
  console.log(`${index + 1}/${candidates.length} raw=${entry.rawIndex} search=${result.search.output?.[0]?.classification ?? 'invocation_failure'} detail=${result.detail?.output?.classification ?? '-'}`);
}
