import { stripVTControlCharacters } from "node:util";
import { createRedactedOutput, redactLogLine } from "./log-redaction.mjs";

// Child tools own their log content, but not the parent terminal's cursor.
export function cleanOutput(text) {
  return stripVTControlCharacters(String(text)).replace(/\r/g, "\n").replace(/[\x00-\x08\x0b-\x1f\x7f]/g, "");
}

export function createDevChildOutput({ terminal, scope, writeLog, onLine = () => {}, maxTailChars = 64 * 1024 }) {
  let tail = "", hiddenTail = "";
  const accept = (chunk, isStderr) => {
    const text = cleanOutput(chunk).split("\n").map(redactLogLine).join("\n");
    tail = (tail + text).slice(-maxTailChars);
    writeLog(text);
    for (const line of text.split("\n")) if (line) onLine(line);
    if (!terminal.child(text, { scope, isStderr })) hiddenTail = (hiddenTail + text).slice(-maxTailChars);
  };
  const stdout = createRedactedOutput((chunk) => accept(chunk, false));
  const stderr = createRedactedOutput((chunk) => accept(chunk, true));
  return { stdout, stderr, tail: () => tail, hiddenTail: () => hiddenTail, end() { stdout.end(); stderr.end(); } };
}

function duration(ms) {
  return ms < 60_000 ? `${(ms / 1000).toFixed(1)}s` : `${Math.floor(ms / 60_000)}m ${Math.floor(ms / 1000) % 60}s`;
}

const graphemes = new Intl.Segmenter(undefined, { granularity: "grapheme" });
const spinner = ["⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"];
const palette = { muted: "90", brand: "32", success: "32", warn: "33", error: "31", strong: "1" };
// Punctuation that must not open a wrapped line; the preceding character moves down with it.
const closingPunctuation = /^[，。、；：！？）》」』】〉”’…·,.;:!?)\]}%]$/u;
const profileNames = { "web-dev": "Web 开发", "launcher-dev": "Launcher 开发", build: "构建产物" };
// Sub-steps shorter than this stay out of a finished stage's breakdown.
const breakdownMs = 1000;

function cellWidth(character) {
  if (/^[\p{Mark}\p{Cf}]+$/u.test(character)) return 0;
  if (/[\p{Emoji_Presentation}️⃣]/u.test(character)) return 2;
  const cp = character.codePointAt(0);
  return cp >= 0x1100 && (cp <= 0x115f || cp === 0x2329 || cp === 0x232a
    || (cp >= 0x2e80 && cp <= 0xa4cf && cp !== 0x303f)
    || (cp >= 0xac00 && cp <= 0xd7a3) || (cp >= 0xf900 && cp <= 0xfaff)
    || (cp >= 0xfe10 && cp <= 0xfe19) || (cp >= 0xfe30 && cp <= 0xfe6f)
    || (cp >= 0xff01 && cp <= 0xff60) || (cp >= 0xffe0 && cp <= 0xffe6)
    || (cp >= 0x20000 && cp <= 0x3fffd)) ? 2 : 1;
}

function textWidth(text) {
  let width = 0;
  for (const { segment } of graphemes.segment(stripVTControlCharacters(text))) width += cellWidth(segment);
  return width;
}

function fitLine(text, columns) {
  if (textWidth(text) <= columns) return text;
  let width = 0, result = "";
  for (const { segment } of graphemes.segment(text)) {
    width += cellWidth(segment);
    if (width > columns - 1) break;
    result += segment;
  }
  return columns > 0 ? result + "…" : "";
}

function wrapLine(text, columns) {
  const lines = [];
  let current = [], width = 0;
  for (const { segment } of graphemes.segment(text)) {
    const cells = cellWidth(segment);
    if (current.length && width + cells > columns) {
      const carry = closingPunctuation.test(segment) && current.length > 1 ? [current.pop()] : [];
      lines.push(current.map((part) => part.segment).join(""));
      current = carry;
      width = carry.reduce((sum, part) => sum + part.cells, 0);
    }
    current.push({ segment, cells });
    width += cells;
  }
  lines.push(current.map((part) => part.segment).join(""));
  return lines;
}

const safeText = (value) => redactLogLine(cleanOutput(value)).replace(/\t/g, "    ");
const singleLine = (value) => safeText(value).replace(/\s+/g, " ");

function describeWatch({ profile, serverReload, pluginDev }) {
  const watching = profile !== "build";
  const server = watching && serverReload === "watch" ? "Server 改动后自动重启" : "Server 改动需重新启动";
  const plugins = pluginDev === "off" ? "不同步开发插件"
    : watching && pluginDev === "watch" ? "插件改动后自动同步" : "插件仅在启动时同步";
  return `${server} · ${plugins}`;
}

export function createDevConsole({
  stdout = process.stdout, stderr = process.stderr, env = process.env,
  now = Date.now, record = () => {}, interval = setInterval, clear = clearInterval,
  startupLogLimit = 120, startupLogChars = 64 * 1024,
} = {}) {
  const interactive = Boolean(stdout.isTTY && stderr.isTTY && env.TERM !== "dumb" && !env.CI);
  const color = interactive && env.NO_COLOR === undefined;
  const tasks = new Map();
  let timer, frame = 0, visibleWidths = [];
  let mode = "stream", sessionStarted = false, summaryShown = false, logHeadingShown = false;
  let bufferedChars = 0, omittedLogs = 0, startupWarnings = 0, lastDiagnostic;
  const startupLogs = [];
  const columns = () => Math.max(12, stdout.columns || 80);
  const contentWidth = () => Math.min(96, columns() - 1);
  const stageWidth = () => Math.min(72, contentWidth());
  // ANSI palette colors follow the terminal's light/dark theme and user contrast settings.
  const paint = (value, role) => color ? `\x1b[${palette[role]}m${value}\x1b[0m` : value;
  const erase = () => {
    if (!visibleWidths.length) return "";
    // A narrower window may reflow a previously drawn row into several physical rows.
    const rows = visibleWidths.reduce((count, width) => count + Math.max(1, Math.ceil(width / columns())), 0);
    visibleWidths = [];
    return "\r\x1b[2K" + "\x1b[1A\r\x1b[2K".repeat(rows - 1);
  };
  // Stages and their sub-steps share one right edge for durations; both titles start after four cells.
  const alignedLine = (lead, title, suffix, titleRole) => {
    const width = stageWidth() - 4;
    const text = fitLine(title, width - suffix.length - 2);
    const gap = Math.max(2, width - textWidth(text) - suffix.length);
    return `${lead}${paint(text, titleRole)}${" ".repeat(gap)}${paint(suffix, "muted")}`;
  };
  const stageLine = (icon, label, elapsed, role) => alignedLine(`  ${paint(icon, role)} `, label, duration(elapsed), "strong");
  const progressLine = (task, showLabel) => {
    const width = contentWidth() - 4;
    const label = showLabel ? fitLine(task.label, Math.min(24, width - 12)) : "";
    const count = `${task.completed}/${task.total}`;
    const fixed = textWidth(label) + (label ? 2 : 0) + 2 + count.length;
    const size = Math.max(4, Math.min(20, width - fixed));
    const room = width - fixed - size - 2;
    const item = task.item && room >= 6 ? fitLine(task.item, room) : "";
    const done = Math.floor(size * task.completed / task.total);
    return "    " + (label ? paint(label, "muted") + "  " : "")
      + paint("━".repeat(done), "brand") + paint("─".repeat(size - done), "muted")
      + "  " + paint(count, "muted") + (item ? "  " + paint(item, "muted") : "");
  };
  const render = () => {
    if (!interactive || !tasks.size) return;
    const activeTasks = [...tasks.values()];
    const root = activeTasks[0], deepest = activeTasks.at(-1);
    const progress = activeTasks.findLast((task) => task.total > 0);
    const lines = [stageLine(spinner[frame++ % spinner.length], root.label, now() - root.started, "brand")];
    if (progress) lines.push(progressLine(progress, progress !== root));
    const detail = deepest !== root && deepest !== progress ? deepest.label : progress ? undefined : root.notes.at(-1);
    if (detail) lines.push("    " + paint(fitLine(detail, contentWidth() - 4), "muted"));
    const clearing = erase();
    // Keep the cursor on an empty row: terminals may truncate the cursor's own
    // row on resize instead of reflowing it like the preceding content.
    visibleWidths = [...lines.map(textWidth), 0];
    stdout.write(clearing + lines.join("\r\n") + "\r\n");
  };
  const print = (lines, output = stdout) => {
    lastDiagnostic = undefined;
    const clearing = erase();
    if (clearing) stdout.write(clearing);
    output.write(lines.join("\n") + "\n");
    render();
  };
  // Redirected output keeps whole lines so paths and commands stay copyable.
  const indented = (text, role) => (interactive ? wrapLine(text, Math.max(1, contentWidth() - 4)) : [text]).map((line) => "    " + (role ? paint(line, role) : line));
  // The startup script's own messages carry the name of their log, logs/dev/start/.
  const write = (message, { scope = "start", level = "info", timestamp, persist = true, diagnosticGroup } = {}) => {
    const safe = safeText(message);
    if (persist) record(safe, level);
    if (level === "debug") return;
    const parsedTime = timestamp ? Date.parse(timestamp) : NaN;
    const time = new Date(Number.isFinite(parsedTime) ? parsedTime : now()).toISOString().slice(11, 19) + "Z";
    const source = singleLine(scope);
    const badge = level === "error" ? "✗" : level === "warn" ? "!" : "·";
    const role = level === "error" || level === "warn" ? level : "muted";
    const output = level === "error" || level === "warn" ? stderr : stdout;
    if (mode === "startup") {
      if (level === "error" || level === "warn") {
        const continued = diagnosticGroup && lastDiagnostic?.group === diagnosticGroup && now() - lastDiagnostic.at < 500;
        if (!continued) startupWarnings++;
        // The orchestrator's own diagnostics need no source label.
        const label = `${level === "error" ? "启动错误" : "启动告警"}${source && source !== "start" ? ` · ${source}` : ""}`;
        const lines = safe.trimEnd().split("\n").flatMap((line) => indented(line));
        print([...(continued ? [] : [`  ${paint(badge, role)} ${paint(fitLine(label, contentWidth() - 4), role)}`]), ...lines], output);
        lastDiagnostic = { group: diagnosticGroup, at: now() };
      } else {
        const root = tasks.values().next().value;
        if (root) { root.notes.push(singleLine(safe)); if (root.notes.length > 4) root.notes.shift(); render(); }
        else print(safe.trimEnd().split("\n").flatMap((line) => indented(line, "muted")));
      }
      return;
    }
    if (!interactive) {
      print(safe.trimEnd().split("\n").map((line) => `${time} ${badge} ${source}  ${line}`), output);
      return;
    }
    const narrow = columns() < 64;
    const sourceWidth = narrow ? Math.max(1, contentWidth() - 16) : 12;
    const shortSource = fitLine(source, sourceWidth);
    const sourceLabel = narrow ? shortSource : shortSource.padEnd(sourceWidth + shortSource.length - textWidth(shortSource));
    const prefix = `  ${paint(time, "muted")} ${paint(badge, role)} ${paint(sourceLabel, role)}`;
    const indent = narrow ? 4 : 28;
    const lines = narrow ? [prefix.trimEnd()] : [];
    for (const paragraph of safe.trimEnd().split("\n")) {
      for (const line of wrapLine(paragraph, Math.max(1, columns() - indent - 1))) {
        lines.push(!narrow && !lines.length ? prefix + "  " + line : " ".repeat(indent) + line);
      }
    }
    print(lines, output);
  };
  const logHeading = (title) => {
    if (logHeadingShown) return;
    logHeadingShown = true;
    print(["", "  " + paint(title, "strong") + "  " + paint("UTC", "muted"), ""]);
  };
  const flushStartup = (title = "进程日志", open = false) => {
    if (mode !== "startup") return;
    mode = "stream";
    if (open || startupLogs.length || omittedLogs) logHeading(title);
    if (omittedLogs) write(`启动期间另有 ${omittedLogs} 条普通日志，完整内容见 logs/dev/。`, { persist: false });
    for (const entry of startupLogs) write(entry.message, entry.options);
    startupLogs.length = 0;
    bufferedChars = 0;
    omittedLogs = 0;
  };
  const runtime = (message, options) => {
    // File logging happens before this bounded, presentation-only startup queue.
    // Preserve receive time for unstructured output as well as source timestamps.
    const sourceTime = options.timestamp ? Date.parse(options.timestamp) : NaN;
    const entry = { message: safeText(message), options: { ...options, scope: singleLine(options.scope || "start"), timestamp: new Date(Number.isFinite(sourceTime) ? sourceTime : now()).toISOString(), persist: false } };
    if (mode !== "startup" || options.level === "warn" || options.level === "error") {
      write(entry.message, entry.options);
      return;
    }
    entry.size = entry.message.length + entry.options.scope.length + String(options.level).length;
    if (entry.size > startupLogChars || startupLogLimit <= 0) { omittedLogs++; return; }
    while (startupLogs.length && (startupLogs.length >= startupLogLimit || bufferedChars + entry.size > startupLogChars)) {
      bufferedChars -= startupLogs.shift().size;
      omittedLogs++;
    }
    startupLogs.push(entry);
    bufferedChars += entry.size;
  };
  // A finished stage lists its slower direct sub-steps, so a slow start shows where the time went.
  const breakdown = (task) => task.steps.filter((step) => step.state === "error" || step.elapsed >= breakdownMs);
  const start = (label) => {
    const token = Symbol(label);
    const parent = [...tasks.values()].at(-1);
    const nested = Boolean(parent);
    const task = { label: singleLine(label), started: now(), completed: 0, total: 0, item: "", notes: [], steps: [], parent };
    tasks.set(token, task);
    record(`${task.label}：开始`, "debug");
    if (!interactive && !nested) {
      if (mode === "startup") print([`  … ${task.label}`]);
      else write(`${task.label}…`, { persist: false });
    }
    if (interactive && !timer) {
      timer = interval(render, 125);
      timer.unref?.();
    }
    render();
    const end = (result, state) => {
      if (!tasks.delete(token)) return;
      const clearing = erase();
      if (clearing) stdout.write(clearing);
      if (!tasks.size) { clear(timer); timer = undefined; }
      const level = state === "error" ? "error" : "info";
      const elapsed = now() - task.started;
      const label = result ? `${task.label} · ${singleLine(result)}` : task.label;
      result = singleLine(result ?? "完成");
      const message = `${task.label} · ${result} · ${duration(elapsed)}`;
      record(message, level);
      if (parent && !parent.parent) parent.steps.push({ label, elapsed, state });
      const steps = breakdown(task);
      if (nested) render();
      else if (!interactive || (sessionStarted && mode === "stream")) {
        // Stage completions are orchestration events, not startup diagnostics.
        if (mode === "startup") print([`  ${state === "error" ? "✗" : state === "success" ? "✓" : "○"} ${message}`,
          ...steps.map((step) => `    ${step.label} · ${duration(step.elapsed)}`),
          ...task.notes.map((note) => "    " + note)], state === "error" ? stderr : stdout);
        else write(message, { level, persist: false });
      } else print([stageLine(state === "success" ? "✓" : state === "error" ? "✗" : "○", label, elapsed, state),
        ...steps.map((step) => alignedLine("    ", step.label, duration(step.elapsed), step.state === "error" ? "error" : "muted")),
        ...task.notes.flatMap((note) => indented(note, "muted"))], state === "error" ? stderr : stdout);
    };
    return {
      progress(completed, total, item = "") {
        task.total = Math.max(0, total);
        task.completed = Math.max(0, Math.min(completed, task.total));
        task.item = item ? singleLine(item) : "";
        render();
      },
      finish(result) { end(result, "success"); },
      cancel() { end("已取消", "muted"); },
      supersede() { end("源码已变化", "muted"); },
      fail() { end("失败", "error"); },
    };
  };
  // Label/value rows share the widest label's column.
  const detailRows = (rows) => {
    const labelWidth = Math.max(...rows.map(([label]) => textWidth(label)));
    return rows.flatMap(([label, value]) => wrapLine(singleLine(value), Math.max(1, contentWidth() - labelWidth - 6))
      .map((line, index) => "    " + (index ? " ".repeat(labelWidth) : paint(label + " ".repeat(labelWidth - textWidth(label)), "muted")) + "  " + line));
  };
  const onResize = () => render();
  if (interactive) stdout.on?.("resize", onResize);
  return {
    write, start,
    intro({ profile, nodeVersion, serverReload, pluginDev }) {
      sessionStarted = true;
      mode = "startup";
      const description = singleLine(`${profileNames[profile] ?? profile} · Node ${nodeVersion}`);
      const watch = describeWatch({ profile, serverReload, pluginDev });
      record(`RayleaBot · ${description} · ${watch} · 日志：logs/dev/`, "info");
      if (!interactive) { print([`RayleaBot 开发环境 · ${description}`, `${watch} · 日志：logs/dev/`, "启动准备"]); return; }
      print(["", "  " + paint("RayleaBot", "brand") + "  " + paint("开发环境", "strong"),
        ...detailRows([["模式", description], ["监听", watch]]), "", "  " + paint("启动准备", "strong")]);
    },
    summary({ elapsedMs, webURL, serverURL, skipLaunch }) {
      if (summaryShown) return;
      summaryShown = true;
      const hint = skipLaunch ? "已跳过 Launcher，开发进程随之结束" : "关闭 Launcher 或按 Ctrl+C 结束";
      const rows = [...(webURL ? [["管理界面", webURL]] : []), ...(serverURL ? [["Server", serverURL]] : []), ["日志", "logs/dev/"]];
      const warning = startupWarnings ? `启动期间有 ${startupWarnings} 条告警，见上方` : "";
      record(`启动准备完成 · ${duration(elapsedMs)} · ${hint}${rows.filter(([label]) => label !== "日志").map(([label, value]) => ` · ${label}：${singleLine(value)}`).join("")}`, "info");
      if (!interactive) {
        print(["", `启动准备完成 · ${duration(elapsedMs)} · ${hint}`,
          ...rows.map(([label, value]) => `${label}：${singleLine(value)}`), ...(warning ? [warning] : [])]);
        flushStartup("进程日志", !skipLaunch);
        return;
      }
      print(["", `  ${paint("✓", "success")} ${paint("启动准备完成", "strong")}  ${paint(duration(elapsedMs), "muted")}`,
        ...detailRows(rows),
        ...(warning ? [`    ${paint("!", "warn")} ${paint(warning, "warn")}`] : []),
        ...indented(hint, "muted")]);
      flushStartup("进程日志", !skipLaunch);
    },
    abortStartup() { flushStartup("启动诊断", true); },
    async phase(label, operation) {
      const task = start(label);
      try { const result = await operation(task); task.finish(); return result; }
      catch (error) {
        if (error?.code === "DEV_SHUTDOWN") task.cancel();
        else if (error?.code === "DEV_INPUT_CHANGED") task.supersede();
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
        let description = entry.msg;
        if (entry.component === "launcher" && level === "info") {
          if (entry.Wails && entry.Compiler) description = `桌面构建 · Wails ${entry.Wails} · ${entry.Compiler}`;
          else if (entry.WebView2) description = `运行环境 · ${entry.Branding || entry.Name || entry.GOOS || "Windows"} · WebView2 ${entry.WebView2}`;
        }
        const message = `${subject ? subject + " · " : ""}${description}${details.length ? " · " + details.join(" · ") : ""}`;
        runtime(message, { scope: entry.component || scope, level, timestamp: typeof entry.ts === "string" ? entry.ts : undefined });
        return true;
      }
      // Successful build chatter stays in the build log; tool warnings remain visible.
      if (line.startsWith("$ ")) return false;
      const diagnostic = isStderr || /^\s*(?:[!▲⚠]\s*)?(?:WARN(?:ING)?\b|ERROR\b|\[?(?:warn(?:ing)?|error)\]?[:\s])/im.test(line);
      if (scope === "build" && !diagnostic) return false;
      runtime(line, { scope, level: diagnostic ? "warn" : "info", diagnosticGroup: diagnostic ? `${scope}:plain` : undefined });
      return true;
    },
    close() {
      const clearing = erase();
      if (clearing) stdout.write(clearing);
      clear(timer);
      timer = undefined;
      tasks.clear();
      if (interactive) stdout.off?.("resize", onResize);
      flushStartup("启动期间的进程输出");
    },
  };
}
