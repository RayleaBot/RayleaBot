import { stripVTControlCharacters } from "node:util";
import { createRedactedOutput, redactLogLine } from "./log-redaction.mjs";

// Child tools own their log content, but not the parent terminal's cursor.
export function cleanOutput(text) {
  return stripVTControlCharacters(String(text)).replace(/\r/g, "\n").replace(/[\x00-\x08\x0b-\x1f\x7f]/g, "");
}

export function createDevChildOutput({ terminal, scope, writeLog, maxTailChars = 64 * 1024 }) {
  let tail = "", hiddenTail = "";
  const accept = (chunk, isStderr) => {
    const text = redactLogLine(cleanOutput(chunk));
    tail = (tail + text).slice(-maxTailChars);
    writeLog(text);
    if (!terminal.child(text, { scope, isStderr })) hiddenTail = (hiddenTail + text).slice(-maxTailChars);
  };
  const stdout = createRedactedOutput((chunk) => accept(chunk, false));
  const stderr = createRedactedOutput((chunk) => accept(chunk, true));
  return { stdout, stderr, tail: () => tail, hiddenTail: () => hiddenTail, end() { stdout.end(); stderr.end(); } };
}

function duration(ms) {
  return ms < 60_000 ? `${(ms / 1000).toFixed(1)}s` : `${Math.floor(ms / 60_000)}m ${Math.floor(ms / 1000) % 60}s`;
}

function fitLine(text, columns) {
  let width = 0, result = "";
  for (const character of text) {
    width += character.codePointAt(0) > 255 ? 2 : 1;
    if (width > columns - 2) return result + "…";
    result += character;
  }
  return result;
}

export function createDevConsole({
  stdout = process.stdout, stderr = process.stderr, env = process.env,
  now = Date.now, record = () => {}, interval = setInterval, clear = clearInterval,
} = {}) {
  const interactive = Boolean(stdout.isTTY && stderr.isTTY && env.TERM !== "dumb" && !env.CI);
  const color = interactive && env.NO_COLOR === undefined;
  const tasks = new Map();
  let timer, frame = 0, visible = false;
  const paint = (value, code) => color ? `\x1b[${code}m${value}\x1b[0m` : value;
  const erase = () => {
    if (visible) stdout.write("\r\x1b[2K");
    visible = false;
  };
  const render = () => {
    if (!interactive || !tasks.size) return;
    const active = [...tasks.values()].at(-1);
    const root = tasks.values().next().value;
    const progress = [...tasks.values()].findLast((task) => task.total > 0);
    const bar = progress ? ` [${"━".repeat(Math.floor(12 * progress.completed / progress.total))}${"·".repeat(12 - Math.floor(12 * progress.completed / progress.total))}] ${progress.completed}/${progress.total}` : "";
    const label = active === root || progress ? active.label : `${root.label} › ${active.label}`;
    const line = `  ${["⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"][frame++ % 10]}${bar} ${label}`;
    const elapsed = ` · ${duration(now() - root.started)}`;
    erase();
    stdout.write(paint(fitLine(line, (stdout.columns || 80) - elapsed.length) + elapsed, "36"));
    visible = true;
  };
  const write = (message, { scope = "dev", level = "info", timestamp, persist = true } = {}) => {
    const safe = redactLogLine(cleanOutput(message));
    if (persist) record(safe, level);
    if (level === "debug") return;
    erase();
    const parsedTime = timestamp ? Date.parse(timestamp) : NaN;
    const time = new Date(Number.isFinite(parsedTime) ? parsedTime : now()).toISOString().slice(11, 19) + "Z";
    const source = redactLogLine(cleanOutput(scope)).replace(/\s+/g, " ");
    const badge = level === "error" ? "✗" : level === "warn" ? "!" : "·";
    const code = level === "error" ? "31" : level === "warn" ? "33" : "36";
    const output = level === "error" || level === "warn" ? stderr : stdout;
    for (const line of safe.trimEnd().split("\n")) {
      output.write(`${paint(time, "90")} ${paint(badge + " " + source, code)}  ${line}\n`);
    }
    render();
  };
  const start = (label) => {
    const token = Symbol(label);
    const nested = tasks.size > 0;
    const task = { label: redactLogLine(cleanOutput(label)), started: now(), completed: 0, total: 0 };
    tasks.set(token, task);
    record(`${task.label}：开始`, "debug");
    if (!interactive && !nested) write(`${task.label}…`, { persist: false });
    if (interactive && !timer) {
      timer = interval(render, 100);
      timer.unref?.();
    }
    render();
    const end = (result, level) => {
      if (!tasks.delete(token)) return;
      erase();
      if (!tasks.size) { clear(timer); timer = undefined; }
      const message = `${task.label} · ${result} · ${duration(now() - task.started)}`;
      if (!nested) write(message, { level });
      else record(message, level);
      render();
    };
    return {
      progress(completed, total) {
        task.total = Math.max(0, total);
        task.completed = Math.max(0, Math.min(completed, task.total));
        render();
      },
      finish(result = "完成") { end(result, "info"); },
      fail() { end("失败", "error"); },
    };
  };
  return {
    write, start,
    async phase(label, operation) {
      const task = start(label);
      try { const result = await operation(task); task.finish(); return result; }
      catch (error) {
        if (error?.code === "DEV_SHUTDOWN") task.finish("已取消");
        else if (error?.code === "DEV_INPUT_CHANGED") task.finish("源码已变化");
        else task.fail();
        throw error;
      }
    },
    child(text, { scope, isStderr = false } = {}) {
      const line = cleanOutput(text).trim();
      if (!line) return false;
      let entry;
      try { entry = JSON.parse(line); } catch { /* Plain tool diagnostics. */ }
      if (entry && typeof entry.msg === "string" && typeof entry.level === "string") {
        const level = entry.level.toLowerCase();
        if (level === "debug" || (scope === "build" && level === "info")) return false;
        const subject = entry.plugin_label || entry.plugin_name || entry.plugin_id || entry.template_id;
        const details = [...new Set([entry.error_code || entry.code, entry.error, entry.err, entry.detail])].filter((value) => typeof value === "string" && value && value !== entry.msg);
        const message = `${subject ? subject + " · " : ""}${entry.msg}${details.length ? " · " + details.join(" · ") : ""}`;
        write(message, { scope: entry.component || scope, level, timestamp: typeof entry.ts === "string" ? entry.ts : undefined, persist: false });
        return true;
      }
      // Successful build chatter stays in the build log; tool warnings remain visible.
      if (scope === "build" && line.startsWith("$ ")) return false;
      if (scope === "build" && !isStderr && !/^\s*(?:[!▲⚠]\s*)?(?:WARN(?:ING)?\b|ERROR\b|\[?(?:warn(?:ing)?|error)\]?[:\s])/im.test(line)) return false;
      write(line, { scope, level: isStderr && scope === "build" ? "warn" : "info", persist: false });
      return true;
    },
    close() {
      erase();
      clear(timer);
      timer = undefined;
      tasks.clear();
    },
  };
}
