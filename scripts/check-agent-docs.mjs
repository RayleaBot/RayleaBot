#!/usr/bin/env node
// scripts/check-agent-docs.mjs
// Checks AGENTS.md, CLAUDE.md, and .agents/skills/**/SKILL.md for structural issues.

import { spawnSync } from "child_process";
import { readFileSync, existsSync } from "fs";
import { join, dirname, relative, resolve } from "path";
import { fileURLToPath } from "url";

const __filename = fileURLToPath(import.meta.url);
const __dirname = dirname(__filename);
const ROOT = resolve(__dirname, "..");

const issues = [];
const secretCandidates = new Set();

function addIssue(file, message) {
  issues.push(`${relative(ROOT, file)}: ${message}`);
}

// ── Collect target files ───────────────────────────────────────────────────

const agentsFiles = [];
const claudeFiles = [];
const skillFiles = [];

function listRepositoryCandidates() {
  const result = spawnSync(
    "git",
    ["-c", `safe.directory=${ROOT}`, "ls-files", "--cached", "--others", "--exclude-standard", "-z"],
    { cwd: ROOT, encoding: "utf8", windowsHide: true },
  );
  if (result.error) {
    console.error(`agent-docs check could not run git ls-files: ${result.error.message}`);
    process.exit(1);
  }
  if (result.status !== 0) {
    const detail = result.stderr.trim() || `exit status ${result.status}`;
    console.error(`agent-docs check could not enumerate repository files: ${detail}`);
    process.exit(1);
  }

  return result.stdout.split("\0").filter(Boolean).sort();
}

for (const candidate of listRepositoryCandidates()) {
  const normalized = candidate.replace(/\\/g, "/");
  const fullPath = resolve(ROOT, candidate);
  if (!existsSync(fullPath)) continue;
  if (normalized === "AGENTS.md" || normalized.endsWith("/AGENTS.md")) agentsFiles.push(fullPath);
  if (normalized === "CLAUDE.md" || normalized.endsWith("/CLAUDE.md")) claudeFiles.push(fullPath);
  if (normalized.startsWith(".agents/skills/") && normalized.endsWith("/SKILL.md")) {
    skillFiles.push(fullPath);
  }
}

// ── 1. Every AGENTS.md must have a sibling bridge importing that guide ──────

function hasSiblingAgentImport(content) {
  let fence = null;
  let inComment = false;
  for (const line of content.split(/\r?\n/)) {
    const marker = !inComment && line.match(/^ {0,3}(`{3,}|~{3,})(.*)$/);
    if (marker) {
      if (!fence) {
        fence = { char: marker[1][0], length: marker[1].length };
      } else if (marker[1][0] === fence.char && marker[1].length >= fence.length && !marker[2].trim()) {
        fence = null;
      }
      continue;
    }
    if (fence || (!inComment && /^(?: {4}|\t)/.test(line))) continue;

    // Comments in code examples are literal. Mask real comments without joining
    // fragments into an import that was not present in the source.
    let visible = "";
    let remaining = line;
    while (remaining) {
      const delimiter = inComment ? "-->" : "<!--";
      const index = remaining.indexOf(delimiter);
      if (index === -1) {
        if (!inComment) visible += remaining;
        break;
      }
      if (!inComment) visible += remaining.slice(0, index);
      visible += " ";
      remaining = remaining.slice(index + delimiter.length);
      inComment = !inComment;
    }
    if (/^ {0,3}@(?:\.\/)?AGENTS\.md[ \t]*$/.test(visible)) return true;
  }
  return false;
}

for (const file of agentsFiles) {
  const bridge = join(dirname(file), "CLAUDE.md");
  if (!existsSync(bridge)) {
    addIssue(file, "missing sibling CLAUDE.md bridge");
  } else if (!hasSiblingAgentImport(readFileSync(bridge, "utf-8"))) {
    addIssue(bridge, "missing active import of sibling AGENTS.md");
  }
}

// ── 2. Line count budgets ──────────────────────────────────────────────────

function countLines(text) {
  const parts = text.split(/\r?\n/);
  if (parts.length > 0 && parts[parts.length - 1] === "") parts.pop();
  return parts.length;
}

const rootAgents = join(ROOT, "AGENTS.md");
const rootClaude = join(ROOT, "CLAUDE.md");

if (existsSync(rootAgents)) {
  const lines = countLines(readFileSync(rootAgents, "utf-8"));
  if (lines > 150) {
    addIssue(rootAgents, `line count ${lines} exceeds budget 150`);
  }
}

if (existsSync(rootClaude)) {
  const lines = countLines(readFileSync(rootClaude, "utf-8"));
  if (lines > 40) {
    addIssue(rootClaude, `line count ${lines} exceeds budget 40`);
  }
}

const LOCAL_AGENTS_BUDGET = 120;
for (const file of agentsFiles) {
  if (file === rootAgents) continue;
  const lines = countLines(readFileSync(file, "utf-8"));
  if (lines > LOCAL_AGENTS_BUDGET) {
    addIssue(file, `line count ${lines} exceeds budget ${LOCAL_AGENTS_BUDGET}`);
  }
}

const SKILL_BUDGET = 100;
for (const file of skillFiles) {
  const lines = countLines(readFileSync(file, "utf-8"));
  if (lines > SKILL_BUDGET) {
    addIssue(file, `line count ${lines} exceeds budget ${SKILL_BUDGET}`);
  }
}

// ── 3. Backtick path existence ───────────────────────────────────────────────

const SHELL_CHARS = /[|&;<>$(){}[\]`\\]/;
const URL_PROTOCOL = /^[a-zA-Z][a-zA-Z0-9+.-]*:\/\//;
const GLOB_CHARS = /[*?]/;
const CONFIG_KEY = /^[a-zA-Z_][a-zA-Z0-9_]*(\.[a-zA-Z_][a-zA-Z0-9_]*)+$/;
const KNOWN_COMMANDS = /^(?:pnpm|npm|corepack|go|node|python|git|npx|sqlc|make|mkdir|cd|echo|export|unset|env|cat|ls|rm|cp|mv|touch|chmod|curl|wget|docker|bash|sh|pwsh|gofmt|golangci-lint|uv|mise|asdf)\s/;
// Skip web routes like /plugins, /plugins/:id, /login?redirect=...
const WEB_ROUTE = /^\/(?:[a-zA-Z0-9_-]+\/)*[a-zA-Z0-9_:-]*$/;
// Skip env var assignments like KEY=value
const ENV_VAR_ASSIGN = /^[A-Z_][A-Z0-9_]*=.+$/;
// Skip benchmark units like ns/op, allocs/op, MB/s
const BENCHMARK_UNIT = /^[a-zA-Z]+\/(?:op|s|iter|min|max|avg|mean|median|stddev|p50|p90|p95|p99|count|total|rate|bytes|MB|GB|KB|ms|us|ns)$/;

const CODE_BLOCK = /^```/;

function extractBacktickPaths(content) {
  const paths = [];
  const lines = content.split(/\r?\n/);
  let inCodeBlock = false;
  for (const line of lines) {
    if (CODE_BLOCK.test(line)) {
      inCodeBlock = !inCodeBlock;
      continue;
    }
    if (inCodeBlock) continue;
    // Match inline backticks, possibly multiple per line
    const regex = /`([^`]+)`/g;
    let m;
    while ((m = regex.exec(line)) !== null) {
      const raw = m[1].trim();
      // Skip if contains shell characters
      if (SHELL_CHARS.test(raw)) continue;
      // Skip URLs
      if (URL_PROTOCOL.test(raw)) continue;
      // Skip globs
      if (GLOB_CHARS.test(raw)) continue;
      // Skip config keys (dot-separated identifiers)
      if (CONFIG_KEY.test(raw)) continue;
      // Skip known command prefixes
      if (KNOWN_COMMANDS.test(raw)) continue;
      // Skip web routes like /plugins, /plugins/:id
      if (WEB_ROUTE.test(raw)) continue;
      // Skip env var assignments like KEY=value
      if (ENV_VAR_ASSIGN.test(raw)) continue;
      // Skip benchmark units like ns/op, allocs/op
      if (BENCHMARK_UNIT.test(raw)) continue;
      // Skip category lists with spaces around slashes like "main / preload / renderer"
      if (raw.includes(" / ")) continue;
      // Skip standalone filenames without directory separator
      // (likely script/tool names referenced by name, not relative paths)
      if (!raw.includes("/") && !raw.includes("\\")) continue;
      paths.push(raw);
    }
  }
  return paths;
}

for (const file of [...agentsFiles, ...claudeFiles, ...skillFiles]) {
  const content = readFileSync(file, "utf-8");
  const paths = extractBacktickPaths(content);
  for (const p of paths) {
    // Try as relative path from the file's directory, then from root
    const dir = dirname(file);
    const relFromDir = join(dir, p);
    const relFromRoot = join(ROOT, p);
    if (!existsSync(relFromDir) && !existsSync(relFromRoot)) {
      addIssue(file, `backtick path \`${p}\` does not exist`);
    }
  }
}

// ── 4. Secret-like strings ───────────────────────────────────────────────────

const SECRET_KEYWORDS = /\b(secret|token|cookie|password|api_key|credential|auth|ck)\b/i;
const EXPLICIT_FAKE = /\b(fixture-only-secret|example-token|example-secret|fake-secret|test-secret|dummy-token|dummy-secret|placeholder-token|placeholder-secret|mock-token|mock-secret|sample-token|sample-secret|your-token|your-secret|xxx|xxxx|xxxxx|replace-me|changeme|not-set|unset|none|null|undefined|empty|string|number|boolean|true|false)\b/i;
const HEX_OR_BASE64 = /^[A-Fa-f0-9]{16,}$|^[A-Za-z0-9+\/]{20,}={0,2}$/;
const LONG_VALUE = /[:=]\s*['"]?([A-Za-z0-9_\-+\/=]{16,})['"]?/;

function checkSecrets(file, content) {
  const lines = content.split(/\r?\n/);
  for (let i = 0; i < lines.length; i++) {
    const line = lines[i];
    if (!SECRET_KEYWORDS.test(line)) continue;
    if (EXPLICIT_FAKE.test(line)) continue;
    const m = line.match(LONG_VALUE);
    if (m) {
      const val = m[1];
      if (val.length >= 16 && HEX_OR_BASE64.test(val)) {
        secretCandidates.add(val);
        addIssue(file, `possible secret on line ${i + 1} (value redacted)`);
      }
    }
  }
}

for (const file of [...agentsFiles, ...claudeFiles, ...skillFiles]) {
  const content = readFileSync(file, "utf-8");
  checkSecrets(file, content);
}

// ── Report ───────────────────────────────────────────────────────────────────

if (issues.length === 0) {
  console.log("agent-docs check passed");
  process.exit(0);
} else {
  const redactions = [...secretCandidates].sort((left, right) => right.length - left.length);
  for (const issue of issues) {
    let diagnostic = issue;
    for (const value of redactions) diagnostic = diagnostic.replaceAll(value, "[redacted]");
    console.log(diagnostic);
  }
  process.exit(1);
}
