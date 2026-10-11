import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import fs from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import test from "node:test";

const checker = new URL("../check-agent-docs.mjs", import.meta.url);
const skillPath = ".agents/skills/fixture-workflow/SKILL.md";
const skill = "---\nname: fixture-workflow\ndescription: Check synthetic repository fixtures.\n---\n";
const upstreamSkillPath = ".agents/skills/impeccable/SKILL.md";

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

test("discovers skills without a root index", async (t) => {
  const result = await runCheck(t, {
    "docs/CHANGELOGS/AGENTS.md": "# Archive\n",
    [skillPath]: skill,
  });
  assert.equal(result.status, 0, result.output);
});

test("reports missing referenced paths after removing the skill index gate", async (t) => {
  const result = await runCheck(t, {
    [skillPath]: `${skill}\nRead \`docs/missing.md\`.\n`,
  });
  assert.equal(result.status, 1, result.output);
  assert.ok(result.output.includes(skillPath), result.output);
  assert.ok(result.output.includes("docs/missing.md"), result.output);
});

for (const name of [skillPath, upstreamSkillPath]) {
  test(`accepts a long skill: ${name}`, async (t) => {
    const result = await runCheck(t, {
      [name]: `${skill}${"Workflow guidance.\n".repeat(200)}`,
    });
    assert.equal(result.status, 0, result.output);
  });
}

test("still checks referenced paths in a long skill", async (t) => {
  const result = await runCheck(t, {
    [skillPath]: `${skill}${"Workflow guidance.\n".repeat(200)}Read \`docs/missing.md\`.\n`,
  });
  assert.equal(result.status, 1, result.output);
  assert.ok(result.output.includes(skillPath), result.output);
  assert.ok(result.output.includes("docs/missing.md"), result.output);
});

test("still checks and redacts suspected credentials in a long skill", async (t) => {
  const syntheticValue = "a".repeat(32);
  const result = await runCheck(t, {
    [upstreamSkillPath]: `${skill}${"Workflow guidance.\n".repeat(200)}token = ${syntheticValue}\n`,
  });
  assert.equal(result.status, 1, result.output);
  assert.ok(result.output.includes(upstreamSkillPath), result.output);
  assert.match(result.output, /possible secret/);
  assert.ok(!result.output.includes(syntheticValue));
});

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
