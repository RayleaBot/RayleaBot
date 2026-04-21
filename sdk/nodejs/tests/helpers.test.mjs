import assert from 'node:assert/strict';
import { spawn } from 'node:child_process';
import path from 'node:path';
import test from 'node:test';
import { fileURLToPath } from 'node:url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const sdkRoot = path.resolve(__dirname, '..');

async function invokeHelper(callExpression) {
  return await new Promise((resolve, reject) => {
    const script = `
      import { createPlugin } from './dist/index.js';

      const plugin = createPlugin();

      try {
        const result = await (${callExpression});
        process.stderr.write(JSON.stringify({ result }) + '\\n');
        process.exit(0);
      } catch (error) {
        process.stderr.write(JSON.stringify({
          name: error.name,
          message: error.message,
        }) + '\\n');
        process.exit(1);
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
      reject(new Error('timed out waiting for helper action frame'));
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
          type: 'result',
          timestamp: Math.floor(Date.now() / 1000),
          plugin_id: actionFrame.plugin_id,
          request_id: actionFrame.request_id,
          status: 'success',
          data: { ok: true },
        }) + '\n',
      );
      resolveActionFrame = actionFrame;
    });

    let resolveActionFrame = null;

    child.stderr.on('data', (chunk) => {
      stderrBuffer += chunk.toString();
    });

    child.on('exit', (code) => {
      clearTimeout(timeout);
      if (code !== 0) {
        reject(new Error(`child exited with code ${code}: ${stderrBuffer}`));
        return;
      }
      if (!resolveActionFrame) {
        reject(new Error('helper did not emit an action frame'));
        return;
      }
      const lines = stderrBuffer.trim().split(/\r?\n/).filter(Boolean);
      resolve({
        actionFrame: resolveActionFrame,
        result: lines.length > 0 ? JSON.parse(lines.at(-1)).result : undefined,
      });
    });
  });
}

async function invokeHelperError(callExpression) {
  return await new Promise((resolve, reject) => {
    const script = `
      import { createPlugin } from './dist/index.js';

      const plugin = createPlugin();

      try {
        await (${callExpression});
        process.stderr.write(JSON.stringify({ unexpected: 'success' }) + '\\n');
        process.exit(1);
      } catch (error) {
        process.stderr.write(JSON.stringify({
          name: error.name,
          message: error.message,
        }) + '\\n');
        process.exit(0);
      }
    `;

    const child = spawn(process.execPath, ['--input-type=module', '--eval', script], {
      cwd: sdkRoot,
      stdio: ['ignore', 'ignore', 'pipe'],
    });

    let stderrBuffer = '';
    const timeout = setTimeout(() => {
      child.kill();
      reject(new Error('timed out waiting for helper validation error'));
    }, 3000);

    child.on('error', (error) => {
      clearTimeout(timeout);
      reject(error);
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
        reject(new Error('helper did not report the validation error'));
        return;
      }
      resolve(JSON.parse(lines.at(-1)));
    });
  });
}

test('messageForwardGet emits the frozen action name', async () => {
  const { actionFrame } = await invokeHelper(
    "plugin.messageForwardGet('evt-1', { forwardId: 'forward-001', timeoutMs: 1000 })",
  );

  assert.equal(actionFrame.action, 'message.forward.get');
  assert.equal(actionFrame.parent_request_id, 'evt-1');
  assert.deepEqual(actionFrame.data, { forward_id: 'forward-001' });
});

test('napcatGroupSignSet emits the frozen provider action name', async () => {
  const { actionFrame } = await invokeHelper(
    "plugin.napcatGroupSignSet('evt-2', 'group-10001', { timeoutMs: 1000 })",
  );

  assert.equal(actionFrame.action, 'provider.napcat.group.sign.set');
  assert.deepEqual(actionFrame.data, { group_id: 'group-10001' });
});

test('governance helpers emit the frozen action names', async () => {
  let result = await invokeHelper(
    "plugin.governanceBlacklistRead('evt-3', { timeoutMs: 1000 })",
  );
  assert.equal(result.actionFrame.action, 'governance.blacklist.read');
  assert.deepEqual(result.actionFrame.data, {});

  result = await invokeHelper(
    "plugin.governanceBlacklistWrite('evt-4', 'upsert', { entryType: 'user', targetId: '10001', reason: 'manual_review', timeoutMs: 1000 })",
  );
  assert.equal(result.actionFrame.action, 'governance.blacklist.write');
  assert.deepEqual(result.actionFrame.data, {
    operation: 'upsert',
    entry_type: 'user',
    target_id: '10001',
    reason: 'manual_review',
  });

  result = await invokeHelper(
    "plugin.governanceWhitelistWrite('evt-5', 'set_enabled', { enabled: true, timeoutMs: 1000 })",
  );
  assert.equal(result.actionFrame.action, 'governance.whitelist.write');
  assert.deepEqual(result.actionFrame.data, {
    operation: 'set_enabled',
    enabled: true,
  });

  result = await invokeHelper(
    "plugin.governanceCommandPolicyRead('evt-6', { timeoutMs: 1000 })",
  );
  assert.equal(result.actionFrame.action, 'governance.command_policy.read');
  assert.deepEqual(result.actionFrame.data, {});
});

test('fileGroupFsDelete rejects when both folderId and fileId are missing', async () => {
  const error = await invokeHelperError(
    "plugin.fileGroupFsDelete('evt-7', 'group-10001')",
  );

  assert.equal(error.name, 'Error');
  assert.match(error.message, /requires folderId or fileId/);
});

test('governance helpers reject missing required fields', async () => {
  const blacklistError = await invokeHelperError(
    "plugin.governanceBlacklistWrite('evt-8', 'upsert', { entryType: 'user', reason: 'missing-target' })",
  );
  assert.equal(blacklistError.name, 'Error');
  assert.match(blacklistError.message, /requires entryType, targetId, and reason/);

  const whitelistError = await invokeHelperError(
    "plugin.governanceWhitelistWrite('evt-9', 'set_enabled', { timeoutMs: 1000 })",
  );
  assert.equal(whitelistError.name, 'Error');
  assert.match(whitelistError.message, /requires enabled/);
});
