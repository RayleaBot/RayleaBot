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

// ── 1. Bridge check: any top-level dir with AGENTS.md must have CLAUDE.md ──

const topLevelDirs = agentsFiles
  .map((file) => relative(ROOT, file).replace(/\\/g, "/").split("/"))
  .filter((parts) => parts.length === 2)
  .map((parts) => parts[0]);

for (const dir of topLevelDirs) {
  const hasAgents = existsSync(join(ROOT, dir, "AGENTS.md"));
  const hasClaude = existsSync(join(ROOT, dir, "CLAUDE.md"));
  if (hasAgents && !hasClaude) {
    addIssue(join(ROOT, dir, "AGENTS.md"), `missing sibling CLAUDE.md bridge`);
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

// ── 2b. Skills list consistency ─────────────────────────────────────────────

function extractMarkdownSection(text, heading) {
  const lines = text.split(/\r?\n/);
  const start = lines.findIndex((line) => line.trim() === `## ${heading}`);
  if (start === -1) return null;
  const body = [];
  for (let i = start + 1; i < lines.length; i += 1) {
    if (/^##\s/.test(lines[i])) break;
    body.push(lines[i]);
  }
  return body.join("\n");
}

const skillsDir = join(ROOT, ".agents", "skills");
const diskSkills = skillFiles
  .map((file) => relative(skillsDir, file).replace(/\\/g, "/").split("/"))
  .filter((parts) => parts.length === 2 && parts[1] === "SKILL.md")
  .map((parts) => parts[0]);

if (existsSync(rootAgents)) {
  const skillsSection = extractMarkdownSection(readFileSync(rootAgents, "utf-8"), "Skills");
  if (skillsSection === null) {
    if (diskSkills.length > 0) {
      addIssue(rootAgents, "missing ## Skills section while .agents/skills/ has entries");
    }
  } else {
    const listedSkills = new Set(
      [...skillsSection.matchAll(/`([^`]+)`/g)].map((m) => m[1])
    );
    for (const name of diskSkills) {
      if (!listedSkills.has(name)) {
        addIssue(rootAgents, `skill \`${name}\` is missing from the Skills section`);
      }
    }
    for (const name of listedSkills) {
      if (!diskSkills.includes(name)) {
        addIssue(rootAgents, `Skills section lists \`${name}\` but .agents/skills/${name}/SKILL.md does not exist`);
      }
    }
  }
}

// ── 3. Backtick path existence ───────────────────────────────────────────────

const SHELL_CHARS = /[|&;<>$(){}[\]`\\]/;
const URL_PROTOCOL = /^[a-zA-Z][a-zA-Z0-9+.-]*:\/\//;
const GLOB_CHARS = /[*?]/;
const CONFIG_KEY = /^[a-zA-Z_][a-zA-Z0-9_]*(\.[a-zA-Z_][a-zA-Z0-9_]*)+$/;
const KNOWN_COMMANDS = /^(pnpm|npm|go|node|python|git|npx|yarn|tsc|vite|eslint|prettier|sqlc|mkdir|cd|echo|export|unset|env|cat|ls|rm|cp|mv|touch|chmod|chown|curl|wget|ssh|scp|docker|kubectl|make|cmake|gcc|g\+\+|clang|rustc|cargo|pip|pipenv|poetry|conda|java|javac|gradle|mvn|dotnet|php|composer|ruby|gem|bundle|rake|perl|lua|julia|dart|flutter|deno|bun|esbuild|rollup|webpack|parcel|turbo|nx|jest|vitest|mocha|ava|tap|playwright|cypress|pytest|unittest|nose|tox|nox|flake8|black|isort|mypy|pylint|bandit|gofmt|golint|staticcheck|govulncheck|revive|errcheck|ineffassign|misspell|structcheck|varcheck|deadcode|gocyclo|gocognit|interfacer|unconvert|unparam|safesql|lll|wsl|gci|goimports|gofumpt|golines|gomnd|nestif|nilerr|noctx|nolintlint|paralleltest|prealloc|promlinter|rowserrcheck|sqlclosecheck|stylecheck|tagliatelle|tenv|testpackage|thelper|tparallel|whitespace|wrapcheck|wsl)\b/;
const EXTENSIONS = /\.(md|txt|json|yaml|yml|toml|sql|go|ts|tsx|js|jsx|mjs|cjs|py|rs|java|kt|scala|rb|php|cs|cpp|c|h|hpp|swift|dart|lua|sh|bat|cmd|ps1|dockerfile|ini|cfg|conf|xml|html|css|scss|sass|less|vue|svelte|svg|png|jpg|jpeg|gif|webp|ico|pdf|zip|tar|gz|bz2|7z|wasm|so|dll|dylib|exe|bin|log|patch|diff|graphql|proto|thrift|avro|parquet|orc|csv|tsv|xls|xlsx|doc|docx|ppt|pptx|mp3|mp4|wav|ogg|webm|mkv|avi|mov|flv|wmv|mpg|mpeg|m4v|m4a|aac|flac|alac|wma|aiff|opus|mid|midi|ac3|dts|eac3|mlp|thd|wavpack|ape|tta|ofs|ofs2|ofs3|spx|speex|celt|silk|amr|awb|evrc|evrcb|evrcwb|evrcnw|smv|qcelp|vmr|g722|g7221|g7222|g726|g729|ilbc|lpc10|codec2|opus|vorbis|theora|vp8|vp9|av1|h264|h265|hevc|mpeg2|mpeg4|avc|svc|mvc|jvt|jct|itu|iso|iec|itu-t|itu-r|ietf|w3c|ecma|ansi|ieee|iso|iec|jis|gb|astm|din|bs|en|csn|gost|ost|r|gost-r|tr|tu|sn|csn|pn|une|uni|nf|nf-en|nf-p|xp|fd|fdp|fdr|fda|fdt|fdx|fdz|fda|fdp|fdr|fds|fdt|fdv|fdw|fdx|fdy|fdz)$/i;
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
      // Skip pure words without extension or path separator
      if (!raw.includes("/") && !raw.includes("\\") && !EXTENSIONS.test(raw)) continue;
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
        addIssue(file, `possible secret on line ${i + 1}: ${line.trim()}`);
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
  for (const issue of issues) {
    console.log(issue);
  }
  process.exit(1);
}
