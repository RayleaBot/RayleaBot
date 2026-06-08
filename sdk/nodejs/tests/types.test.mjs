import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import path from 'node:path';
import test from 'node:test';
import { fileURLToPath } from 'node:url';

import {
  flashFileSegment,
  markdownSegment,
  recordSegment,
  shakeSegment,
} from '../dist/types.js';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const sdkRoot = path.resolve(__dirname, '..');

test('named segment builders cover flash_file and passthrough families', () => {
  assert.deepEqual(recordSegment({ file: 'voice.amr' }), {
    type: 'record',
    data: { file: 'voice.amr' },
  });
  assert.deepEqual(markdownSegment('## title'), {
    type: 'markdown',
    data: { content: '## title' },
  });
  assert.deepEqual(flashFileSegment({ name: 'clip.zip' }), {
    type: 'flash_file',
    data: { name: 'clip.zip' },
  });
  assert.deepEqual(shakeSegment({ strength: 'full' }), {
    type: 'shake',
    data: { strength: 'full' },
  });
});

test('generated declaration files include meta fields and new helpers', async () => {
  const typesText = await fs.readFile(path.join(sdkRoot, 'dist', 'types.d.ts'), 'utf8');
  const indexText = await fs.readFile(path.join(sdkRoot, 'dist', 'index.d.ts'), 'utf8');

  assert.match(typesText, /flash_file/);
  assert.match(typesText, /meta_event_type\?: string;/);
  assert.match(typesText, /interval\?: number;/);
  assert.match(typesText, /status\?: Record<string, unknown>;/);
  assert.match(typesText, /bot\?: Bot;/);
  assert.match(typesText, /export interface BilibiliPayload/);
  assert.match(typesText, /kind: 'live' \| 'dynamic';/);
  assert.match(typesText, /service: 'live' \| 'video' \| 'image_text' \| 'article' \| 'repost';/);
  assert.match(typesText, /live_event\?: 'started' \| 'ended';/);
  assert.match(typesText, /bilibili\?: BilibiliPayload;/);

  assert.match(indexText, /messageForwardGet/);
  assert.match(indexText, /fileGroupFsDelete/);
  assert.match(indexText, /napcatGroupSignSet/);
  assert.match(indexText, /export declare class PluginEventContext/);
  assert.match(indexText, /export declare class RayleaBotPlugin/);
  assert.match(indexText, /readonly botId: string;/);
  assert.match(indexText, /readonly capabilities: string\[\];/);
  assert.match(indexText, /readonly superAdmins: string\[\];/);
  assert.match(typesText, /permissions\?: \{/);
  assert.match(typesText, /super_admins\?: string\[\];/);

  const contextDeclaration = indexText.slice(
    indexText.indexOf('export declare class PluginEventContext'),
    indexText.indexOf('export declare class RayleaBotPlugin'),
  );
  for (const helper of [
    'storageFileRead',
    'storageFileDelete',
    'storageFileList',
    'messageGet',
    'messageForwardSend',
    'groupList',
    'fileGroupFsDelete',
    'providerAction',
    'luckylilliaFriendGroupsGet',
  ]) {
    assert.match(contextDeclaration, new RegExp(`${helper}\\(`));
  }
});
