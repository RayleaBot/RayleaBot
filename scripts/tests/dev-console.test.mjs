import assert from "node:assert/strict";
import test from "node:test";
import { createDevChildOutput, createDevConsole } from "../dev-console.mjs";

function fixture({ tty = false, env = {}, columns = 120 } = {}) {
  const out = [], errors = [], records = [];
  let tick, scheduled = 0, cleared = 0;
  const terminal = createDevConsole({
    stdout: { isTTY: tty, columns, write: (text) => out.push(text) },
    stderr: { isTTY: tty, write: (text) => errors.push(text) }, env,
    now: () => Date.parse("2026-09-23T12:34:56Z"),
    record: (text, level) => records.push({ text, level }),
    interval(callback) { tick = callback; scheduled++; return { unref() {} }; },
    clear(timer) { if (timer) cleared++; },
  });
  return { terminal, out, errors, records, tick: () => tick?.(), scheduled: () => scheduled, cleared: () => cleared };
}

test("build output is retained in full while warnings remain visible and failure context excludes displayed diagnostics", () => {
  const f = fixture(), log = [];
  const output = createDevChildOutput({ terminal: f.terminal, scope: "build", writeLog: (text) => log.push(text) });
  output.stdout.write(Buffer.from("\x1b[36minternal/frontend/dist/assets/app.js  150 kB\x1b[0m\n"));
  output.stdout.write(Buffer.from('WARN unsupported setting\n{"level":"WARN","msg":"invalid plugin","err":"bad manifest"}\n'));
  output.stderr.write(Buffer.from("compiler.go:5: unexpected token\n"));
  output.stderr.write(Buffer.from("$ vue-tsc --noEmit && vite build\n"));
  output.stdout.write(Buffer.from("failure details without final newline"));
  output.end();
  assert.match(log.join(""), /internal\/frontend\/dist\/assets\/app.js/);
  assert.doesNotMatch(f.out.join("") + f.errors.join(""), /assets\/app.js/);
  assert.match(f.out.join(""), /WARN unsupported setting/);
  assert.match(f.errors.join(""), /bad manifest/);
  assert.match(f.errors.join(""), /unexpected token/);
  assert.doesNotMatch(f.errors.join(""), /vue-tsc/);
  assert.match(log.join(""), /vue-tsc/);
  assert.match(output.hiddenTail(), /failure details without final newline/);
  assert.doesNotMatch(output.hiddenTail(), /bad manifest|unexpected token|unsupported setting/);
  assert.match(output.tail(), /unexpected token/);
  assert.doesNotMatch(log.join(""), /\x1b/);
});

test("split UTF-8 and secrets are redacted before log, terminal and failure tails", () => {
  const f = fixture(), log = [];
  const output = createDevChildOutput({ terminal: f.terminal, scope: "server", writeLog: (text) => log.push(text) });
  const bytes = Buffer.from('{"level":"ERROR","msg":"同步失败 token=fixture-secret","plugin_id":"fixture.plugin","err":"manifest invalid"}\n');
  for (const byte of bytes) output.stderr.write(Buffer.from([byte]));
  output.end();
  for (const text of [log.join(""), f.errors.join(""), output.tail()]) {
    assert.doesNotMatch(text, /fixture-secret|�/);
    assert.match(text, /同步失败/);
  }
  assert.match(f.errors.join(""), /fixture.plugin/);
  assert.match(f.errors.join(""), /manifest invalid/);
});

test("runtime events keep their identity and are never deduplicated just by message", () => {
  const f = fixture();
  for (const plugin_id of ["one", "two", "one"]) f.terminal.child(JSON.stringify({ level: "INFO", msg: "started", plugin_id }), { scope: "server" });
  assert.equal(f.out.length, 3);
  assert.match(f.out[0], /one/);
  assert.match(f.out[1], /two/);
});

test("untrusted runtime metadata cannot move the cursor and source times are displayed in UTC", () => {
  const f = fixture();
  f.terminal.child(JSON.stringify({ level: "ERROR", msg: "failure", component: "\x1b[2Jserver\nforged", ts: "2026-09-23T20:34:56+08:00" }), { scope: "server" });
  assert.match(f.errors.join(""), /^12:34:56Z/);
  assert.equal(f.errors.join("").trim().split("\n").length, 1);
  assert.doesNotMatch(f.errors.join(""), /\x1b/);
});

for (const options of [{ tty: false }, { tty: true, env: { CI: "1" } }, { tty: true, env: { TERM: "dumb" } }]) {
  test(`non-interactive output contains no animation: ${JSON.stringify(options)}`, async () => {
    const f = fixture(options);
    await f.terminal.phase("prepare", async (task) => {
      task.progress(1, 3);
      await f.terminal.phase("child", async () => {});
    });
    f.terminal.close();
    assert.equal(f.scheduled(), 0);
    assert.equal(f.out.length, 2);
    assert.doesNotMatch(f.out.join(""), /\x1b|\r|child/);
    assert.ok(f.records.some((record) => record.text.includes("child")));
  });
}

test("interactive progress only advances on completed work and stops on failure", async () => {
  const f = fixture({ tty: true });
  await assert.rejects(f.terminal.phase("prepare", async (task) => {
    task.progress(0, 4);
    f.tick(); f.tick();
    assert.match(f.out.at(-1), /0\/4/);
    task.progress(2, 4);
    assert.match(f.out.at(-1), /2\/4/);
    f.terminal.write("warning", { level: "warn" });
    assert.match(f.errors.join(""), /warning/);
    throw new Error("fixture failure");
  }), /fixture failure/);
  assert.equal(f.scheduled(), 1);
  assert.equal(f.cleared(), 1);
  assert.match(f.errors.join(""), /失败/);
  assert.doesNotMatch(f.records.map((record) => record.text).join(""), /\x1b|⠋|━━/);
  f.terminal.close();
  const length = f.out.length;
  f.tick();
  assert.equal(f.out.length, length);
});

test("NO_COLOR keeps progress but removes color and close releases an unfinished task", () => {
  const f = fixture({ tty: true, env: { NO_COLOR: "" }, columns: 40 });
  f.terminal.start("构建一个很长很长很长的开发插件名称");
  assert.match(f.out.join(""), /…/);
  assert.doesNotMatch(f.out.join(""), /\x1b\[\d+m/);
  f.terminal.close();
  assert.equal(f.cleared(), 1);
  assert.equal(f.out.at(-1), "\r\x1b[2K");
});

test("deliberate shutdown ends the active phase without reporting a build failure", async () => {
  const f = fixture({ tty: true });
  await assert.rejects(f.terminal.phase("build", async () => {
    throw Object.assign(new Error("stopping"), { code: "DEV_SHUTDOWN" });
  }), { code: "DEV_SHUTDOWN" });
  assert.match(f.out.join(""), /已取消/);
  assert.equal(f.errors.length, 0);
  assert.equal(f.cleared(), 1);
});

test("a superseded build releases progress without a spurious failure warning", async () => {
  const f = fixture();
  await assert.rejects(f.terminal.phase("plugin", async () => {
    throw Object.assign(new Error("retry"), { code: "DEV_INPUT_CHANGED" });
  }), { code: "DEV_INPUT_CHANGED" });
  assert.equal(f.errors.length, 0);
  assert.match(f.out.join(""), /源码已变化/);
});

test("failure tails have bounded memory even when child output is large", () => {
  const f = fixture();
  const output = createDevChildOutput({ terminal: f.terminal, scope: "build", writeLog() {}, maxTailChars: 64 });
  output.stdout.write(Buffer.from("x".repeat(1024) + "\nlast diagnostic"));
  output.end();
  assert.equal(output.tail().length, 64);
  assert.equal(output.hiddenTail().length, 64);
  assert.match(output.hiddenTail(), /last diagnostic$/);
});
