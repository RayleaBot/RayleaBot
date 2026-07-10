import { rm } from 'node:fs/promises';
import { spawnSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';

const sdkRoot = fileURLToPath(new URL('../', import.meta.url));
const dist = fileURLToPath(new URL('../dist', import.meta.url));
const tsc = fileURLToPath(new URL('../node_modules/typescript/bin/tsc', import.meta.url));

await rm(dist, { recursive: true, force: true });
const result = spawnSync(process.execPath, [tsc, '--project', sdkRoot], {
  cwd: sdkRoot,
  stdio: 'inherit',
});

if (result.error) {
  throw result.error;
}
process.exitCode = result.status ?? 1;
