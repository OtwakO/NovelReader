/* global process, console, URL */
import { spawn } from 'node:child_process';
import { fileURLToPath } from 'node:url';

const [, , tool, ...args] = process.argv;
if (!tool) {
  console.error('Usage: node scripts/run-node-tool.mjs <vite|vitest> [...args]');
  process.exit(2);
}

const entrypoints = {
  vite: new URL('../node_modules/vite/bin/vite.js', import.meta.url),
  vitest: new URL('../node_modules/vitest/vitest.mjs', import.meta.url),
};
const entrypoint = entrypoints[tool];
if (!entrypoint) {
  console.error(`Unsupported Node tool: ${tool}`);
  process.exit(2);
}

const inheritedOptions = process.env.NODE_OPTIONS?.trim();
const webStorageOption = '--no-experimental-webstorage';
const nodeOptions = inheritedOptions?.includes(webStorageOption)
  ? inheritedOptions
  : [inheritedOptions, webStorageOption].filter(Boolean).join(' ');

const child = spawn(process.execPath, [fileURLToPath(entrypoint), ...args], {
  stdio: 'inherit',
  env: { ...process.env, NODE_OPTIONS: nodeOptions },
});
child.on('error', (error) => {
  console.error(error);
  process.exit(1);
});
child.on('exit', (code, signal) => {
  if (signal) process.kill(process.pid, signal);
  else process.exit(code ?? 1);
});
