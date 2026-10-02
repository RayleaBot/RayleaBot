import assert from "node:assert/strict";
import { EventEmitter } from "node:events";
import test from "node:test";
import { createDevChildOutput, createDevConsole } from "../dev-console.mjs";
import { createServerReadinessWatch } from "../start-dev-support.mjs";

function fixture({ tty = false, env = {}, columns = 120, ...options } = {}) {
  const out = [], errors = [], records = [];
  let tick, scheduled = 0, cleared = 0, time = Date.parse("2026-09-23T12:34:56Z");
  const stdout = Object.assign(new EventEmitter(), { isTTY: tty, columns, write: (text) => out.push(text) });
  const terminal = createDevConsole({
    ...options,
    stdout,
    stderr: { isTTY: tty, write: (text) => errors.push(text) }, env,
    now: () => time,
    record: (text, level) => records.push({ text, level }),
    interval(callback) { tick = callback; scheduled++; return { unref() {} }; },
    clear(timer) { if (timer) cleared++; },
  });
  return { terminal, stdout, out, errors, records, advance: (ms) => { time += ms; }, tick: () => tick?.(), scheduled: () => scheduled, cleared: () => cleared };
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
  assert.match(f.errors.join(""), /WARN unsupported setting/);
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

test("structured child output preserves JSONL record boundaries in files and failure tails", () => {
  const f = fixture(), log = [];
  const output = createDevChildOutput({ terminal: f.terminal, scope: "server", writeLog: (text) => log.push(text) });
  const first = { component: "runtime_prepare", stage: "download", downloaded_bytes: 1 };
  const second = { component: "runtime_prepare", stage: "download", downloaded_bytes: 2 };
  output.stdout.write(Buffer.from(JSON.stringify(first) + "\n"));
  output.stdout.write(Buffer.from(JSON.stringify(second) + "\n"));
  output.end();
  for (const text of [log.join(""), output.tail()]) {
    assert.deepEqual(text.trim().split("\n").map((line) => JSON.parse(line)), [first, second]);
  }
});

test("readiness sees each complete progress record independently of the bounded diagnostic tail", () => {
  const f = fixture();
  let current = 0;
  const watch = createServerReadinessWatch({ timeoutMs: 30, prepareStallMs: 100, prepareMaxMs: 500, now: () => current });
  const output = createDevChildOutput({ terminal: f.terminal, scope: "server", maxTailChars: 32, writeLog() {}, onLine: (line) => watch.observe(line) });
  const progress = Buffer.from('{ "component": "runtime_prepare", "stage": "download", "downloaded_bytes": 1 }\n');
  output.stdout.write(progress.subarray(0, 20));
  assert.equal(watch.isPreparing(), false);
  output.stdout.write(progress.subarray(20));
  assert.equal(watch.isPreparing(), true);
  current = 90;
  output.stderr.write(Buffer.from('{"component":"storage","msg":"unrelated"}\n'));
  current = 100;
  assert.equal(watch.hasExpired(), true);
  output.end();
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
  assert.match(f.out.at(-1), /^\r\x1b\[2K/);
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

test("narrow diagnostic wrapping preserves the entire message and grapheme clusters", () => {
  const f = fixture({ tty: true, columns: 40, env: { NO_COLOR: "" } });
  const message = "构建诊断：开发者👩‍💻的 cafe\u0301 模板处理失败，请检查资源文件后重新运行。";
  f.terminal.write(message, { scope: "build", level: "error" });
  const lines = f.errors.join("").trimEnd().split("\n");
  assert.equal(lines.slice(1).map((line) => line.slice(4)).join(""), message);
  assert.match(f.errors.join(""), /👩‍💻/u);
  assert.match(f.errors.join(""), /e\u0301/u);
  f.terminal.close();
});

test("resize listeners are released and completed tasks cannot redraw after close", () => {
  const f = fixture({ tty: true });
  const task = f.terminal.start("build");
  task.progress(1, 3);
  f.stdout.columns = 40;
  f.stdout.emit("resize");
  assert.equal(f.stdout.listenerCount("resize"), 1);
  task.finish();
  f.terminal.close();
  assert.equal(f.stdout.listenerCount("resize"), 0);
  const length = f.out.length;
  f.stdout.emit("resize");
  f.tick();
  assert.equal(f.out.length, length);
});

test("plain startup summary exposes the selected plugin mode without leaking credentials", () => {
  const f = fixture();
  f.terminal.intro({ profile: "web-dev", nodeVersion: "v26.7.0", serverReload: "watch", pluginDev: "off" });
  f.terminal.summary({ elapsedMs: 2000, webURL: "http://localhost/?setup_token=fixture-secret", skipLaunch: false });
  const text = f.out.join("") + f.records.map((entry) => entry.text).join("\n");
  assert.match(text, /源码同步关闭/);
  assert.match(text, /localhost/);
  assert.doesNotMatch(text, /fixture-secret|\x1b/);
  f.terminal.close();
});

test("task outcomes are sanitized before reaching the startup log", () => {
  const f = fixture();
  f.terminal.start("build").finish("\x1b[2Jtoken=fixture-secret");
  const text = f.out.join("") + f.records.map((entry) => entry.text).join("\n");
  assert.doesNotMatch(text, /fixture-secret|\x1b/);
});

function beginStartup(f) {
  f.terminal.intro({ profile: "web-dev", nodeVersion: "v26.7.0", serverReload: "watch", pluginDev: "watch" });
}

for (const tty of [false, true]) {
  test(`startup process output is filed immediately and displayed only after the summary (TTY=${tty})`, () => {
    const f = fixture({ tty, env: { NO_COLOR: "" } }), file = [];
    beginStartup(f);
    const task = f.terminal.start("Server");
    const output = createDevChildOutput({ terminal: f.terminal, scope: "server", writeLog: (text) => file.push(text) });
    output.stdout.write(Buffer.from('{"level":"INFO","msg":"FIRST_PROCESS_EVENT","ts":"2026-09-23T12:34:50Z","component":"runtime"}\n'));
    output.stdout.write(Buffer.from('PLAIN_PROCESS_EVENT\n'));
    output.end();
    assert.match(file.join(""), /FIRST_PROCESS_EVENT/);
    assert.doesNotMatch(f.out.join(""), /FIRST_PROCESS_EVENT|PLAIN_PROCESS_EVENT/);
    f.advance(5000);
    task.finish();
    f.terminal.summary({ elapsedMs: 5000, skipLaunch: false });
    f.terminal.child('{"level":"INFO","msg":"LATE_PROCESS_EVENT","component":"runtime"}', { scope: "server" });
    const displayed = f.out.join("");
    assert.ok(displayed.indexOf("启动准备完成") < displayed.indexOf("进程日志"));
    assert.ok(displayed.indexOf("进程日志") < displayed.indexOf("FIRST_PROCESS_EVENT"));
    assert.ok(displayed.indexOf("FIRST_PROCESS_EVENT") < displayed.indexOf("PLAIN_PROCESS_EVENT"));
    assert.ok(displayed.indexOf("PLAIN_PROCESS_EVENT") < displayed.indexOf("LATE_PROCESS_EVENT"));
    assert.match(displayed, /12:34:50Z/);
    assert.match(displayed, /12:34:56Z/);
    assert.equal(output.hiddenTail(), "");
    f.terminal.summary({ elapsedMs: 5000, skipLaunch: false });
    f.terminal.close();
    assert.equal(f.out.join("").split("FIRST_PROCESS_EVENT").length - 1, 1);
    assert.equal(f.out.join("").split("进程日志").length - 1, 1);
  });
}

test("startup warnings and stderr diagnostics are immediate and never replayed", () => {
  const f = fixture();
  beginStartup(f);
  f.terminal.child('{"level":"INFO","msg":"BUFFERED_CONTEXT"}', { scope: "server" });
  f.terminal.child('{"level":"WARN","msg":"ACCOUNT_WARNING"}', { scope: "server" });
  f.terminal.child("RUNTIME_STDERR", { scope: "web", isStderr: true });
  assert.match(f.errors.join(""), /ACCOUNT_WARNING/);
  assert.match(f.errors.join(""), /RUNTIME_STDERR/);
  assert.doesNotMatch(f.out.join(""), /BUFFERED_CONTEXT/);
  f.terminal.summary({ elapsedMs: 1000, skipLaunch: false });
  assert.match(f.out.join(""), /BUFFERED_CONTEXT/);
  assert.doesNotMatch(f.out.join(""), /ACCOUNT_WARNING|RUNTIME_STDERR/);
  f.terminal.close();
});

test("startup replay is bounded but the child log retains omitted output", () => {
  const f = fixture({ startupLogLimit: 2, startupLogChars: 80 }), file = [];
  beginStartup(f);
  const output = createDevChildOutput({ terminal: f.terminal, scope: "server", writeLog: (text) => file.push(text) });
  output.stdout.write(Buffer.from("EARLIEST\nSECOND\nTHIRD\n" + "x".repeat(120) + "\n"));
  output.end();
  f.terminal.summary({ elapsedMs: 1000, skipLaunch: false });
  assert.match(file.join(""), /EARLIEST/);
  assert.match(file.join(""), /x{120}/);
  assert.doesNotMatch(f.out.join(""), /EARLIEST|x{120}/);
  assert.match(f.out.join(""), /SECOND/);
  assert.match(f.out.join(""), /THIRD/);
  assert.match(f.out.join(""), /2 条普通日志/);
  f.terminal.close();
});

test("failed or cancelled startup releases queued context without a success claim", () => {
  for (const abort of [true, false]) {
    const f = fixture();
    beginStartup(f);
    f.terminal.child("EARLY_CONTEXT token=fixture-secret", { scope: "server" });
    if (abort) { f.terminal.abortStartup(); f.terminal.write("FAILED_STARTUP", { level: "error" }); }
    f.terminal.close();
    f.terminal.close();
    const text = f.out.join("") + f.errors.join("");
    assert.match(text, /EARLY_CONTEXT/);
    assert.doesNotMatch(text, /启动准备完成|fixture-secret/);
    assert.equal(text.split("EARLY_CONTEXT").length - 1, 1);
  }
});

test("Launcher structured metadata remains useful in the compact process log", () => {
  const f = fixture();
  f.terminal.child(JSON.stringify({ level: "INFO", component: "launcher", msg: "Build Info:", Wails: "v3.0.0", Compiler: "go1.26.6" }), { scope: "launcher" });
  f.terminal.child(JSON.stringify({ level: "INFO", component: "launcher", msg: "Platform Info:", Branding: "Windows 11", WebView2: "153.0" }), { scope: "launcher" });
  assert.match(f.out.join(""), /Wails v3\.0\.0.*go1\.26\.6/);
  assert.match(f.out.join(""), /Windows 11.*WebView2 153\.0/);
  f.terminal.child(JSON.stringify({ level: "ERROR", component: "launcher", msg: "LAUNCHER_START_FAILED", Wails: "v3.0.0", Compiler: "go1.26.6" }), { scope: "launcher" });
  assert.match(f.errors.join(""), /LAUNCHER_START_FAILED/);
});
