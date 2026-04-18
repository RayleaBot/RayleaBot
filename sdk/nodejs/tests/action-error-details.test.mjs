import assert from 'node:assert/strict';
import { spawn } from 'node:child_process';
import path from 'node:path';
import test from 'node:test';
import { fileURLToPath } from 'node:url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const sdkRoot = path.resolve(__dirname, '..');

async function requestLocalActionError(errorFrame) {
  return await new Promise((resolve, reject) => {
    const script = `
      import { requestLocalAction } from './src/protocol.js';

      try {
        await requestLocalAction(
          'helper-plugin',
          'evt-local',
          'logger.write',
          { level: 'warn', message: 'attempt denied' },
          { timeoutMs: 1000 },
        );
        process.stderr.write(JSON.stringify({ unexpected: 'success' }) + '\\n');
        process.exit(1);
      } catch (error) {
        process.stderr.write(JSON.stringify({
          name: error.name,
          code: error.code,
          message: error.message,
          details: error.details,
        }) + '\\n');
        process.exit(0);
      }
    `;

    const child = spawn(process.execPath, ['--input-type=module', '--eval', script], {
      cwd: sdkRoot,
      stdio: ['pipe', 'pipe', 'pipe'],
    });

    let stdoutBuffer = '';
    let stderrBuffer = '';
    let wroteResponse = false;
    const timeout = setTimeout(() => {
      child.kill();
      reject(new Error('timed out waiting for child sdk response'));
    }, 3000);

    child.on('error', (error) => {
      clearTimeout(timeout);
      reject(error);
    });

    child.stdout.on('data', (chunk) => {
      stdoutBuffer += chunk.toString();
      if (wroteResponse) {
        return;
      }
      const newlineIndex = stdoutBuffer.indexOf('\n');
      if (newlineIndex < 0) {
        return;
      }
      wroteResponse = true;
      const actionFrame = JSON.parse(stdoutBuffer.slice(0, newlineIndex));
      child.stdin.end(
        JSON.stringify({
          protocol_version: '1',
          type: 'error',
          timestamp: Math.floor(Date.now() / 1000),
          plugin_id: 'helper-plugin',
          request_id: actionFrame.request_id,
          ...errorFrame,
        }) + '\n',
      );
    });

    child.stderr.on('data', (chunk) => {
      stderrBuffer += chunk.toString();
    });

    child.on('exit', (code) => {
      clearTimeout(timeout);
      if (code !== 0) {
        reject(new Error(`child exited with code ${code}: ${stderrBuffer}`));
        return;
      }
      const lines = stderrBuffer.trim().split(/\r?\n/).filter(Boolean);
      if (lines.length === 0) {
        reject(new Error('child did not report action error'));
        return;
      }
      resolve(JSON.parse(lines.at(-1)));
    });
  });
}

test('requestLocalAction preserves structured error details', async () => {
  const error = await requestLocalActionError({
    code: 'platform.rate_limited',
    message: 'outbound request rejected by policy',
    details: { retry_after_seconds: 30, policy: 'http.egress' },
  });

  assert.equal(error.name, 'ActionError');
  assert.equal(error.code, 'platform.rate_limited');
  assert.deepEqual(error.details, { retry_after_seconds: 30, policy: 'http.egress' });
});

test('requestLocalAction defaults missing error details to an empty object', async () => {
  const error = await requestLocalActionError({
    code: 'permission.scope_violation',
    message: 'capability not granted',
  });

  assert.equal(error.name, 'ActionError');
  assert.equal(error.code, 'permission.scope_violation');
  assert.deepEqual(error.details, {});
});
