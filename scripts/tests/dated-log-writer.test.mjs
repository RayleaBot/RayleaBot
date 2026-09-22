import assert from "node:assert/strict";
import fs from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import test from "node:test";
import { createDatedLogWriter, formatUTCLogDate, formatUTCLogTimestamp, resolveDatedLogPath } from "../start-dev-support.mjs";
import { createRedactedOutput } from "../log-redaction.mjs";

test("log dates and timestamp precision use UTC across timezone and daylight-saving boundaries", () => {
  for (const [input, expected] of [
    ["2026-09-23T07:59:59.123+08:00", "2026-09-22T23:59:59.123000000Z"],
    ["2026-09-22T17:00:00-07:00", "2026-09-23T00:00:00.000000000Z"],
    ["2026-11-01T01:30:00-04:00", "2026-11-01T05:30:00.000000000Z"],
    ["2026-11-01T01:30:00-05:00", "2026-11-01T06:30:00.000000000Z"],
  ]) {
    const instant = new Date(input);
    assert.equal(formatUTCLogTimestamp(instant), expected);
    assert.equal(formatUTCLogDate(instant), expected.slice(0, 10));
  }
});

test("development logs rotate by receipt date while queued writes retain their original day", async (t) => {
  const rootDir = await fs.mkdtemp(path.join(os.tmpdir(), "raylea-dated-logs-"));
  let date = new Date(Date.UTC(2026, 8, 15, 23, 59, 59));
  const types = ["start", "build", "server", "web", "launcher"];
  const writers = types.map((type) => createDatedLogWriter({ rootDir, scope: "dev", type, now: () => date }));
  t.after(async () => {
    await Promise.all(writers.map((writer) => writer.end()));
    await fs.rm(rootDir, { recursive: true, force: true });
  });
  for (const writer of writers) writer.write("before midnight\n");
  date = new Date(Date.UTC(2026, 8, 16, 0, 0, 0));
  for (const writer of writers) writer.write("after midnight\n");
  date = new Date(Date.UTC(2026, 8, 22, 8, 0, 0));
  for (const writer of writers) writer.write("after idle days\n");
  date = new Date(Date.UTC(2026, 8, 15, 23, 59, 59));
  for (const writer of writers) writer.write("after clock correction\n");
  await Promise.all(writers.map((writer) => writer.end()));

  for (const type of types) {
    const directory = path.join(rootDir, "logs", "dev", type);
    assert.deepEqual((await fs.readdir(directory)).sort(), ["2026-09-15.log", "2026-09-16.log", "2026-09-22.log"]);
    assert.equal(await fs.readFile(path.join(directory, "2026-09-15.log"), "utf8"), "before midnight\nafter clock correction\n");
    assert.equal(await fs.readFile(path.join(directory, "2026-09-16.log"), "utf8"), "after midnight\n");
    assert.equal(await fs.readFile(path.join(directory, "2026-09-22.log"), "utf8"), "after idle days\n");
  }

  date = new Date(Date.UTC(2026, 8, 22, 8, 1, 0));
  const restarted = createDatedLogWriter({ rootDir, scope: "dev", type: "server", now: () => date });
  writers.push(restarted);
  restarted.write("restarted process\n");
  await restarted.end();
  assert.equal(await fs.readFile(restarted.path, "utf8"), "after idle days\nrestarted process\n");
});

test("a line split across midnight stays intact and redacted in the day it completes", async (t) => {
  const rootDir = await fs.mkdtemp(path.join(os.tmpdir(), "raylea-dated-lines-"));
  let date = new Date(Date.UTC(2026, 11, 31, 23, 59, 59));
  const writer = createDatedLogWriter({ rootDir, scope: "dev", type: "server", now: () => date });
  const previousPath = writer.path;
  const output = createRedactedOutput((chunk) => writer.write(chunk));
  t.after(async () => {
    await writer.end();
    await fs.rm(rootDir, { recursive: true, force: true });
  });

  output.write(Buffer.from("Cookie: fixture-before-"));
  date = new Date(Date.UTC(2027, 0, 1, 0, 0, 0));
  output.write(Buffer.from("and-after\n消息正常\n末行"));
  output.end();
  await writer.end();

  assert.equal(writer.path, resolveDatedLogPath({ rootDir, scope: "dev", type: "server", date }));
  assert.equal(await fs.readFile(writer.path, "utf8"), "Cookie: [REDACTED]\n消息正常\n末行");
  await assert.rejects(fs.stat(previousPath), { code: "ENOENT" });
});

test("unused writers do not create files and write failures reach the closing caller", async (t) => {
  const rootDir = await fs.mkdtemp(path.join(os.tmpdir(), "raylea-dated-errors-"));
  t.after(() => fs.rm(rootDir, { recursive: true, force: true }));
  const unused = createDatedLogWriter({ rootDir, scope: "dev", type: "web" });
  await unused.end();
  await assert.rejects(fs.stat(path.join(rootDir, "logs")), { code: "ENOENT" });

  await fs.writeFile(path.join(rootDir, "logs"), "directory blocked");
  const blocked = createDatedLogWriter({ rootDir, scope: "dev", type: "server" });
  blocked.write("must not silently disappear\n");
  await assert.rejects(blocked.end(), (error) => ["EEXIST", "ENOTDIR", "ENOENT"].includes(error.code));
});
