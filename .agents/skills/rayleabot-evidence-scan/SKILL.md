---
name: rayleabot-evidence-scan
description: Use when RayleaBot tasks require evidence-only investigation, including daily bug scans, measurable performance regression watches, recent PR/review/commit skill recommendations, automation memory updates, or Windows worktree evidence gathering. Prefer concrete repo evidence, measured artifacts, targeted tests, and local Git fallback over guesses, churn-based conclusions, or generic advice.
---

# RayleaBot Evidence Scan

This skill preserves the evidence boundary for recurring RayleaBot scans. Repository truth still lives in root/local `AGENTS.md`, `contracts/`, formal docs, tests, fixtures, and current Git state.

## Workflow

1. Read root `AGENTS.md` and any closer local `AGENTS.md` for the touched area.
2. Classify the task:
   - `bug-scan`: recent commit or diff review for concrete bugs
   - `performance-watch`: measurable regression check
   - `skill-recommendation`: next skill or learning recommendation from recent work
   - `automation-memory`: memory update for an automation result
3. Gather the smallest evidence set that can answer the task:
   - recent commits, diff, changed files, and ownership boundaries
   - targeted test, build, typecheck, CI, or `git diff --check` output
   - contract, generated type, fixture, example, or doc drift
   - benchmark, trace, profile, timing, or Web Vitals artifacts
4. Report only what the evidence supports.
5. If evidence is missing, state the uncertainty briefly and name the next measurement or check that would resolve it.

## Bug Scans

- Report a bug only when there is concrete evidence: failing command output, compile/typecheck error, test failure, invalid diff, contract drift, or a directly traceable logic regression.
- Prefer the smallest safe fix when implementation is requested.
- Stop at `no concrete bug found` when targeted evidence does not support a bug.
- Do not infer bugs from churn, file count, commit titles, broad risk, or unfamiliar code.

## Performance Watches

- Treat these as measurement tasks, not code-shape reviews.
- Search for measured artifacts such as `Benchmark`, `benchstat`, `ns/op`, `allocs/op`, `trace`, `pprof`, `profile`, `flamegraph`, `Lighthouse`, `performance.mark`, `performance.measure`, `console.time`, `PerformanceObserver`, `Web Vitals`, CPU profile, heap profile, and timing logs.
- If no measured artifacts are found, write `No measurements found`.
- A local one-off benchmark can be described only as an early signal unless it is an established comparable baseline.
- Do not convert diff size, UI complexity, commit names, or one-line changes into a performance verdict.

## Skill Recommendations

- Anchor each recommendation to concrete evidence from PR themes, review comments, commit clusters, touched paths, failures, or recurring fixes.
- If GitHub API or `gh pr list` is blocked by `connectex` or network policy, use local Git history and touched-file clustering instead.
- Avoid generic advice. Each recommendation must name the repeated work pattern and the skill that would reduce it.

## Automation Memory

- Use explicit paths under `<user-home>\.codex\automations\<automation-name>\memory.md`.
- Do not rely on `$CODEX_HOME` or `$env:CODEX_HOME` in this Windows environment.
- Keep memory entries aligned with the user-facing conclusion and the evidence that supports it.

## Windows Worktrees

- If read-only Git commands hit dubious ownership, use `git -c safe.directory=<repo> ...`.
- Do not change global Git config for temporary scans.
- If a verified fix from a scan worktree must be committed to `main`, keep unrelated dirty changes out of the commit.

## Output

- Lead with the conclusion.
- Include the actual reason and verification result.
- Use `No measurements found` exactly for missing performance measurements.
- Do not include speculation, generic risk language, or process narration.
