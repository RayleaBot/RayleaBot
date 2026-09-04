import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import fs from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import test from "node:test";

const checker = new URL("../check-agent-docs.mjs", import.meta.url);
const skillPath = ".agents/skills/fixture-workflow/SKILL.md";
const skill = "---\nname: fixture-workflow\ndescription: Check synthetic repository fixtures.\n---\n";

async function runCheck(t, overrides = {}) {
  const root = await fs.mkdtemp(path.join(os.tmpdir(), "rayleabot-agent-docs-"));
  t.after(async () => {
    const target = await fs.realpath(root);
    const parent = await fs.realpath(os.tmpdir());
    assert.equal(path.dirname(target), parent);
    assert.ok(path.basename(target).startsWith("rayleabot-agent-docs-"));
    await fs.rm(target, { recursive: true, force: true });
  });
  const files = {
    "AGENTS.md": "# Repository\n",
    "CLAUDE.md": "@AGENTS.md\n",
    ...overrides,
  };
  for (const [name, content] of Object.entries(files)) {
    if (content === null) continue;
    const destination = path.join(root, name);
    await fs.mkdir(path.dirname(destination), { recursive: true });
    await fs.writeFile(destination, content);
  }
  await fs.mkdir(path.join(root, "scripts"), { recursive: true });
  await fs.copyFile(checker, path.join(root, "scripts/check-agent-docs.mjs"));
  const options = { cwd: root, encoding: "utf8", windowsHide: true, timeout: 10_000 };
  const init = spawnSync("git", ["init", "--quiet"], options);
  assert.ifError(init.error);
  assert.equal(init.status, 0, init.stderr);
  const result = spawnSync(process.execPath, ["scripts/check-agent-docs.mjs"], options);
  assert.ifError(result.error);
  return { status: result.status, output: `${result.stdout}${result.stderr}`.replaceAll("\\", "/") };
}

test("discovers skills without a root index and accepts bridges with notes", async (t) => {
  const result = await runCheck(t, {
    "CLAUDE.md": "# Client notes\r\n\r\n```md\r\n@other.md\r\n```\r\n\r\n@./AGENTS.md\r\n\r\nUse the repository guide.\r\n",
    "docs/CHANGELOGS/AGENTS.md": "# Archive\n",
    "docs/CHANGELOGS/CLAUDE.md": "@AGENTS.md\n",
    [skillPath]: skill,
  });
  assert.equal(result.status, 0, result.output);
});

for (const [label, example] of [
  ["fenced HTML", "```html\n<!--\n```\n"],
  ["indented HTML", "    <!--\n"],
]) {
  test(`accepts a real import after a ${label} comment example`, async (t) => {
    const result = await runCheck(t, { "CLAUDE.md": `${example}\n@AGENTS.md\n` });
    assert.equal(result.status, 0, result.output);
  });
}

for (const directory of ["", "docs/CHANGELOGS/"]) {
  test(`requires a sibling bridge for ${directory || "root"} AGENTS`, async (t) => {
    const result = await runCheck(t, {
      [`${directory}AGENTS.md`]: "# Scoped guide\n",
      [`${directory}CLAUDE.md`]: null,
    });
    assert.equal(result.status, 1, result.output);
    assert.ok(result.output.includes(`${directory}AGENTS.md`), result.output);
  });
}

const invalidImports = [
  ["different file", "@other.md\n"],
  ["ancestor guide", "@../AGENTS.md\n"],
  ["fenced example", "```md\n@AGENTS.md\n```\n"],
  ["tilde-fenced example", "~~~md\n@AGENTS.md\n~~~\n"],
  ["HTML comment", "<!--\n@AGENTS.md\n-->\n"],
  ["comment-split filename", "@AG<!-- omitted -->ENTS.md\n"],
  ["indented example", "    @AGENTS.md\n"],
];
for (const [label, content] of invalidImports) {
  test(`rejects a bridge containing only an import from ${label}`, async (t) => {
    const result = await runCheck(t, {
      "docs/AGENTS.md": "# Docs\n",
      "docs/other.md": "# Other guide\n",
      "docs/CLAUDE.md": content,
    });
    assert.equal(result.status, 1, result.output);
    assert.ok(result.output.includes("docs/CLAUDE.md"), result.output);
  });
}

test("reports missing referenced paths after removing the skill index gate", async (t) => {
  const result = await runCheck(t, {
    [skillPath]: `${skill}\nRead \`docs/missing.md\`.\n`,
  });
  assert.equal(result.status, 1, result.output);
  assert.ok(result.output.includes(skillPath), result.output);
  assert.ok(result.output.includes("docs/missing.md"), result.output);
});

for (const [name, budget] of [
  ["AGENTS.md", 150],
  ["CLAUDE.md", 40],
  ["docs/AGENTS.md", 120],
  [skillPath, 100],
]) {
  test(`retains the line budget for ${name}`, async (t) => {
    const result = await runCheck(t, {
      "docs/CLAUDE.md": "@AGENTS.md\n",
      [name]: "@AGENTS.md\n".repeat(budget + 1),
    });
    assert.equal(result.status, 1, result.output);
    assert.ok(result.output.includes(name), result.output);
    assert.match(result.output, new RegExp(`\\b${budget + 1}\\b`));
  });
}

test("reports a suspected credential location without echoing its value", async (t) => {
  const syntheticValue = "a".repeat(32);
  const result = await runCheck(t, { "AGENTS.md": `# Repository\n\ntoken = ${syntheticValue}\n` });
  assert.equal(result.status, 1, result.output);
  assert.match(result.output, /AGENTS\.md.*\b3\b/);
  assert.ok(!result.output.includes(syntheticValue));
});

test("redacts detected credentials from other diagnostics on the same input", async (t) => {
  const syntheticValue = `${"a".repeat(24)}/${"b".repeat(24)}`;
  const result = await runCheck(t, { "AGENTS.md": `# Repository\n\n\`token = "${syntheticValue}"\`\n` });
  assert.equal(result.status, 1, result.output);
  assert.match(result.output, /AGENTS\.md.*\b3\b/);
  assert.ok(!result.output.includes(syntheticValue));
});

test("accepts explicit fixture credentials", async (t) => {
  const result = await runCheck(t, { "AGENTS.md": "# Repository\n\ntoken = fixture-only-secret\n" });
  assert.equal(result.status, 0, result.output);
});
