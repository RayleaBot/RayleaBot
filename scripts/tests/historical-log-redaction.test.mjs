import test from "node:test";
import assert from "node:assert/strict";
import fs from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import { redactHistoricalLogs } from "../redact-historical-logs.mjs";

test("historical sanitization only changes confirmed setup values and is idempotent", async (t) => {
  const root = await fs.mkdtemp(path.join(os.tmpdir(), "raylea-log-test-"));
  t.after(() => fs.rm(root, { recursive: true, force: true }));
  const file = path.join(root, "fixture.log");
  const input = 'timestamp error preserved\r\nhttp://localhost/setup#setup_token=fixture-only-setup-secret-123\r\n{"request_id":"req-1","count":9}\r\n';
  await fs.writeFile(file, input);
  assert.equal((await redactHistoricalLogs(root))[0].replacements, 1);
  assert.equal(await fs.readFile(file, "utf8"), input);
  const result = await redactHistoricalLogs(root, true);
  assert.equal(result[0].lines, 3);
  assert.equal(await fs.readFile(file, "utf8"), input.replace("fixture-only-setup-secret-123", "[REDACTED]"));
  assert.deepEqual(await redactHistoricalLogs(root, true), []);
});
