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

function cellWidth(character) {
  if (/^[\p{Mark}\p{Cf}]+$/u.test(character)) return 0;
  if (/[\p{Emoji_Presentation}\uFE0F\u20E3]/u.test(character)) return 2;
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
  let width = 0, line = "";
  for (const { segment } of graphemes.segment(text)) {
    const cells = cellWidth(segment);
    if (line && width + cells > columns) { lines.push(line); line = ""; width = 0; }
    line += segment;
    width += cells;
  }
  lines.push(line);
  return lines;
}

const safeText = (value) => redactLogLine(cleanOutput(value)).replace(/\t/g, "    ");
const singleLine = (value) => safeText(value).replace(/\s+/g, " ");

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
  let bufferedChars = 0, omittedLogs = 0, startupWarning = false, lastDiagnostic;
  const startupLogs = [];
  const columns = () => Math.max(12, stdout.columns || 80);
  const contentWidth = () => Math.min(96, columns() - 1);
  // ANSI palette colors follow the terminal's light/dark theme and user contrast settings.
  const paint = (value, role) => color ? `\x1b[${palette[role]}m${value}\x1b[0m` : value;
  const erase = () => {
    if (!visibleWidths.length) return "";
    // A narrower window may reflow a previously drawn row into several physical rows.
    const rows = visibleWidths.reduce((count, width) => count + Math.max(1, Math.ceil(width / columns())), 0);
    visibleWidths = [];
    return "\r\x1b[2K" + "\x1b[1A\r\x1b[2K".repeat(rows - 1);
  };
  const stageLine = (icon, label, elapsed, role) => {
    const suffix = duration(elapsed);
    const width = Math.min(72, contentWidth());
    const title = fitLine(label, width - suffix.length - 7);
    const gap = Math.max(2, width - 4 - textWidth(title) - suffix.length);
    return `  ${paint(icon, role)} ${paint(title, "strong")}${" ".repeat(gap)}${paint(suffix, "muted")}`;
  };
  const render = () => {
    if (!interactive || !tasks.size) return;
    const activeTasks = [...tasks.values()];
    const root = activeTasks[0], active = activeTasks.at(-1);
    const progress = activeTasks.findLast((task) => task.total > 0);
    const lines = [stageLine(spinner[frame++ % spinner.length], root.label, now() - root.started, "brand")];
    const detail = active !== root ? active.label : root.notes.at(-1);
    if (detail) lines.push("    " + paint(fitLine(detail, contentWidth() - 4), "muted"));
    if (progress) {
      const count = `${progress.completed}/${progress.total}`;
      const size = Math.max(1, Math.min(24, contentWidth() - count.length - 7));
      const done = Math.floor(size * progress.completed / progress.total);
      lines.push(`    ${paint("━".repeat(done), "brand")}${paint("─".repeat(size - done), "muted")}  ${paint(count, "muted")}`);
    }
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
  const write = (message, { scope = "dev", level = "info", timestamp, persist = true, diagnosticGroup } = {}) => {
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
        startupWarning = true;
        const continued = diagnosticGroup && lastDiagnostic?.group === diagnosticGroup && now() - lastDiagnostic.at < 500;
        const label = `${level === "error" ? "启动错误" : "启动告警"} · ${source}`;
        const lines = safe.trimEnd().split("\n").flatMap((line) => wrapLine(line, Math.max(1, contentWidth() - 4))).map((line) => "    " + line);
        print([...(continued ? [] : [`  ${paint(badge, role)} ${paint(fitLine(label, contentWidth() - 4), role)}`]), ...lines], output);
        lastDiagnostic = { group: diagnosticGroup, at: now() };
      } else {
        const root = tasks.values().next().value;
        if (root) { root.notes.push(singleLine(safe)); if (root.notes.length > 4) root.notes.shift(); render(); }
        else print(safe.trimEnd().split("\n").flatMap((line) => wrapLine(line, Math.max(1, contentWidth() - 4))).map((line) => "    " + paint(line, "muted")));
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
    const entry = { message: safeText(message), options: { ...options, scope: singleLine(options.scope || "dev"), timestamp: new Date(Number.isFinite(sourceTime) ? sourceTime : now()).toISOString(), persist: false } };
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
  const start = (label) => {
    const token = Symbol(label);
    const nested = tasks.size > 0;
    const task = { label: singleLine(label), started: now(), completed: 0, total: 0, notes: [] };
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
      const label = result ? `${task.label} · ${singleLine(result)}` : task.label;
      result = singleLine(result ?? "完成");
      const message = `${task.label} · ${result} · ${duration(now() - task.started)}`;
      record(message, level);
      if (nested) render();
      else if (!interactive || (sessionStarted && mode === "stream")) {
        // Stage completions are orchestration events, not startup diagnostics.
        if (mode === "startup") print([`  ${state === "error" ? "✗" : state === "success" ? "✓" : "○"} ${message}`,
          ...task.notes.map((note) => "    " + note)], state === "error" ? stderr : stdout);
        else write(message, { level, persist: false });
      } else print([stageLine(state === "success" ? "✓" : state === "error" ? "✗" : "○", label, now() - task.started, state),
        ...task.notes.flatMap((note) => wrapLine(note, Math.max(1, contentWidth() - 4))).map((line) => "    " + paint(line, "muted"))], state === "error" ? stderr : stdout);
    };
    return {
      progress(completed, total) {
        task.total = Math.max(0, total);
        task.completed = Math.max(0, Math.min(completed, task.total));
        render();
      },
      finish(result) { end(result, "success"); },
      cancel() { end("已取消", "muted"); },
      supersede() { end("源码已变化", "muted"); },
      fail() { end("失败", "error"); },
    };
  };
  const detailLines = (label, value) => {
    const lines = wrapLine(singleLine(value), Math.max(1, contentWidth() - 12));
    return lines.map((line, index) => (index ? "            " : `    ${paint(label.padEnd(6 + label.length - textWidth(label)), "muted")}  `) + line);
  };
  const onResize = () => render();
  if (interactive) stdout.on?.("resize", onResize);
  return {
    write, start,
    intro({ profile, nodeVersion, serverReload, pluginDev }) {
      sessionStarted = true;
      mode = "startup";
      const description = singleLine(`${profile} · Node ${nodeVersion}`);
      const watch = `Server ${serverReload === "watch" ? "监听" : "不监听"} · 插件 ${pluginDev === "watch" ? "监听" : pluginDev === "sync" ? "启动时同步" : "源码同步关闭"}`;
      record(`RayleaBot · ${description} · ${watch} · 日志：logs/dev/`, "info");
      if (!interactive) { print([`RayleaBot · ${description}`, `${watch} · 日志：logs/dev/`, "启动准备"]); return; }
      print(["", "  " + paint("RayleaBot", "brand") + "  " + paint("开发环境", "strong"),
        ...wrapLine(description, contentWidth() - 2).map((line) => "  " + paint(line, "muted")),
        ...wrapLine(watch, contentWidth() - 2).map((line) => "  " + paint(line, "muted")), "", "  " + paint("启动准备", "strong")]);
    },
    summary({ elapsedMs, webURL, skipLaunch }) {
      if (summaryShown) return;
      summaryShown = true;
      const hint = skipLaunch ? "检查结束，未打开 Launcher" : "关闭 Launcher 或按 Ctrl+C 结束";
      record(`启动准备完成 · ${duration(elapsedMs)} · ${hint}${webURL ? ` · 管理面：${singleLine(webURL)}` : ""}`, "info");
      if (!interactive) {
        print(["", `启动准备完成 · ${duration(elapsedMs)} · ${hint}`,
          ...(webURL ? [`管理面：${singleLine(webURL)}`] : []), "日志：logs/dev/"]);
        flushStartup("进程日志", !skipLaunch);
        return;
      }
      print(["", `  ${paint("✓", "success")} ${paint("启动准备完成", "strong")}  ${paint(duration(elapsedMs), "muted")}`,
        ...(webURL ? detailLines("管理面", webURL) : []), ...detailLines("日志", "logs/dev/"),
        ...(startupWarning ? wrapLine("启动期间有告警，详情见上方诊断与日志文件。", contentWidth() - 4).map((line) => "    " + paint(line, "warn")) : []),
        ...wrapLine(hint, contentWidth() - 4).map((line) => "    " + paint(line, "muted"))]);
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
