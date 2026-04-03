# Launcher Homepage Redesign Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rebuild the launcher status homepage into a balanced desktop-style layout that removes card overlap and button collisions without changing launcher behavior.

**Architecture:** Keep the existing `AppShell` status-view information architecture and Fluent UI button usage, but replace the current fragile homepage markup and CSS with a stable hero plus content-grid structure. Drive the change with renderer tests first, then tighten the responsive CSS and regression assertions around the new breakpoint behavior.

**Tech Stack:** React 18, Fluent UI React v9, global `style.css`, Vitest, Testing Library

---

## File Map

- Modify: `launcher/src/renderer/src/AppShell.tsx`
  Responsibility: status homepage JSX structure, button grouping, summary-card order, stable class names for layout rules.
- Modify: `launcher/src/renderer/src/style.css`
  Responsibility: homepage layout, shared card rhythm, balanced hero actions, responsive breakpoints, overflow-safe rules.
- Modify: `launcher/tests/renderer/app-shell.test.tsx`
  Responsibility: renderer-level assertions for the homepage DOM structure and user-visible controls.
- Modify: `launcher/tests/renderer/style-regressions.test.ts`
  Responsibility: stylesheet regression checks that lock in the non-overlapping homepage breakpoints.

## Task 1: Lock the homepage DOM contract with renderer tests

**Files:**
- Modify: `launcher/tests/renderer/app-shell.test.tsx`
- Test: `launcher/tests/renderer/app-shell.test.tsx`

- [ ] **Step 1: Add a failing status-page structure test**

Add a new test next to `renders navigation, hero summary, and environment warning` that renders `activeSection="status"` and asserts the homepage exposes the stable structural hooks the redesign will use.

```tsx
function renderStatusShell() {
  return render(
    <AppShell
      snapshot={snapshot}
      activeSection="status"
      platformLabel="win32-x64"
      settingsDraft={snapshot.settings}
      resolvedSettings={snapshot.resolvedSettings}
      editingSettings={false}
      diagnosticsSummary=""
      busyAction={null}
      controlsDisabled={false}
      isMaximized={false}
      onNavigate={vi.fn()}
      onRefresh={vi.fn()}
      onStart={vi.fn()}
      onStop={vi.fn()}
      onOpenWeb={vi.fn()}
      onRecoveryRecheck={vi.fn()}
      onRuntimeBootstrap={vi.fn()}
      onOpenRecoveryPlugin={vi.fn()}
      onOpenReleasePage={vi.fn()}
      onOpenLogs={vi.fn()}
      onResetAdmin={vi.fn()}
      onBeginEdit={vi.fn()}
      onCancelEdit={vi.fn()}
      onSaveSettings={vi.fn()}
      onUpdateInstallationRoot={vi.fn()}
      onUpdateCloseBehavior={vi.fn()}
      onUpdateAdvancedOverride={vi.fn()}
      onChooseInstallationRoot={vi.fn()}
      onChooseServer={vi.fn()}
      onChooseConfig={vi.fn()}
      onChooseWorkdir={vi.fn()}
      onExit={vi.fn()}
    />,
  );
}

test("renders the balanced status homepage layout", () => {
  const { container } = renderStatusShell();

  expect(container.querySelector(".status-homepage")).not.toBeNull();
  expect(container.querySelector(".status-hero")).not.toBeNull();
  expect(container.querySelector(".status-hero__body")).not.toBeNull();
  expect(container.querySelector(".status-hero__actions")).not.toBeNull();
  expect(container.querySelector(".status-summary-grid")).not.toBeNull();
  expect(container.querySelector(".status-summary-rail")).not.toBeNull();
  expect(container.querySelector(".status-log-panel")).not.toBeNull();
});
```

- [ ] **Step 2: Add a failing action-group assertion**

Assert the primary action and the two secondary actions live in the expected groups so the later CSS work has a stable DOM target.

```tsx
const primaryAction = screen.getByRole("button", { name: "启动 RayleaBot" });
const stopAction = screen.getByRole("button", { name: "停止服务" });
const manageAction = screen.getByRole("button", { name: "管理面板" });

expect(primaryAction.closest(".status-hero__primary-action")).not.toBeNull();
expect(stopAction.closest(".status-hero__secondary-actions")).not.toBeNull();
expect(manageAction.closest(".status-hero__secondary-actions")).not.toBeNull();
```

- [ ] **Step 3: Add a failing lightweight summary-rail assertion**

Check that version, recovery, and warnings render inside the rail container instead of the primary content card.

```tsx
const rail = container.querySelector(".status-summary-rail");
expect(within(rail as HTMLElement).getByText("版本监控")).toBeInTheDocument();
expect(within(rail as HTMLElement).getByText("恢复兼容性")).toBeInTheDocument();
expect(within(rail as HTMLElement).getByText("环境预警")).toBeInTheDocument();
```

- [ ] **Step 4: Run the focused renderer test to verify failure**

Run from `launcher/`: `pnpm test -- tests/renderer/app-shell.test.tsx`

Expected: FAIL because the new homepage class names and grouping do not exist yet.

- [ ] **Step 5: Commit the test-only change**

```bash
git add launcher/tests/renderer/app-shell.test.tsx
git commit -m "test(launcher): cover homepage layout structure"
```

## Task 2: Rebuild the homepage JSX around the approved layout

**Files:**
- Modify: `launcher/src/renderer/src/AppShell.tsx`
- Test: `launcher/tests/renderer/app-shell.test.tsx`

- [ ] **Step 1: Replace the current status-page wrappers with the approved structure**

Refactor only the `activeSection === "status"` block into:

```tsx
<div className="status-homepage">
  <section className="status-hero glass-panel">
    <div className="status-hero__body">{/* status text + alerts */}</div>
    <div className="status-hero__actions">{/* primary + secondary groups */}</div>
  </section>

  <div className="status-summary-grid">
    <div className="status-summary-main">{/* core parameters */}</div>
    <aside className="status-summary-rail">{/* warnings + release + recovery */}</aside>
  </div>

  <article className="status-log-panel panel glass-panel">{/* logs */}</article>
</div>
```

- [ ] **Step 2: Normalize the action markup without changing behavior**

Keep the same click handlers and disabled logic, but move them into explicit groups:

```tsx
<div className="status-hero__primary-action">
  <Button className="frost-button frost-button--primary status-action status-action--primary" ...>
    {primaryActionLabel}
  </Button>
</div>

<div className="status-hero__secondary-actions">
  <Button className="frost-button frost-button--secondary status-action" ...>停止服务</Button>
  <Button className="frost-button frost-button--secondary status-action" ...>管理面板</Button>
</div>
```

- [ ] **Step 3: Keep the summary rail lightweight**

Move environment warnings, version monitoring, and recovery cards into the rail container. Do not add new behaviors, new copy, or new data sources.

- [ ] **Step 4: Run the focused renderer test to verify the JSX contract**

Run from `launcher/`: `pnpm test -- tests/renderer/app-shell.test.tsx`

Expected: PASS for the new homepage structure test.

- [ ] **Step 5: Commit the JSX-only homepage refactor**

```bash
git add launcher/src/renderer/src/AppShell.tsx
git commit -m "refactor(launcher): rebalance homepage structure"
```

## Task 3: Lock the responsive CSS contract with regression tests

**Files:**
- Modify: `launcher/tests/renderer/style-regressions.test.ts`
- Test: `launcher/tests/renderer/style-regressions.test.ts`

- [ ] **Step 1: Add a failing hero-layout regression**

Assert the stylesheet defines the new homepage layout classes and the balanced desktop rules.

```ts
expect(styleSheet).toMatch(/\.status-hero\s*{[^}]*grid-template-columns:\s*minmax\(0,\s*1fr\)\s*minmax\(240px,\s*320px\);/s);
expect(styleSheet).toMatch(/\.status-hero__secondary-actions\s*{[^}]*grid-template-columns:\s*repeat\(2,\s*minmax\(0,\s*1fr\)\);/s);
```

- [ ] **Step 2: Add a failing responsive downgrade regression**

Assert the stylesheet explicitly downgrades the hero and summary grid before collisions can happen.

```ts
expect(styleSheet).toMatch(/@media\s*\(max-width:\s*1200px\)\s*{[^}]*\.status-summary-grid\s*{[^}]*grid-template-columns:\s*1fr;/s);
expect(styleSheet).toMatch(/@media\s*\(max-width:\s*960px\)\s*{[^}]*\.status-hero\s*{[^}]*grid-template-columns:\s*1fr;/s);
expect(styleSheet).toMatch(/@media\s*\(max-width:\s*960px\)\s*{[^}]*\.status-hero__secondary-actions\s*{[^}]*grid-template-columns:\s*1fr;/s);
```

- [ ] **Step 3: Add a failing overflow-safety regression**

Check for the minimum overflow controls that prevent path fields and log content from blowing up the layout.

```ts
expect(styleSheet).toMatch(/\.status-summary-main\s*{[^}]*min-width:\s*0;/s);
expect(styleSheet).toMatch(/\.status-summary-rail\s*{[^}]*min-width:\s*0;/s);
expect(styleSheet).toMatch(/\.status-log-panel\s*{[^}]*min-width:\s*0;/s);
```

- [ ] **Step 4: Run the stylesheet regression test to verify failure**

Run from `launcher/`: `pnpm test -- tests/renderer/style-regressions.test.ts`

Expected: FAIL because the new homepage CSS contract is not implemented yet.

- [ ] **Step 5: Commit the regression-test update**

```bash
git add launcher/tests/renderer/style-regressions.test.ts
git commit -m "test(launcher): cover homepage responsive layout"
```

## Task 4: Implement the homepage CSS system

**Files:**
- Modify: `launcher/src/renderer/src/style.css`
- Test: `launcher/tests/renderer/style-regressions.test.ts`

- [ ] **Step 1: Remove the fragile homepage-specific experimental styles**

Delete or replace the current `status-view-flow`, `hero-card--fancy`, `hero-actions--premium`, `status-grid`, `check-item-mini`, and related experimental homepage blocks that depend on fixed widths without downgrade rules.

- [ ] **Step 2: Add the new stable homepage layout classes**

Implement the homepage CSS around the structure from Task 2:

```css
.status-homepage {
  display: flex;
  flex-direction: column;
  gap: 18px;
}

.status-hero {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(240px, 320px);
  gap: 20px;
  align-items: stretch;
}

.status-summary-grid {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(240px, 300px);
  gap: 18px;
  align-items: start;
}
```

- [ ] **Step 3: Implement the balanced action-area rules**

Use a one-primary plus two-secondary grid that stays stable across widths:

```css
.status-hero__actions {
  display: flex;
  flex-direction: column;
  gap: 10px;
  min-width: 0;
}

.status-hero__secondary-actions {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px;
}

.status-action--primary.fui-Button {
  min-height: 52px;
  justify-content: center;
}
```

- [ ] **Step 4: Implement the responsive downgrade breakpoints**

Add explicit breakpoint rules so the layout collapses before overlap:

```css
@media (max-width: 1200px) {
  .status-summary-grid {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 960px) {
  .status-hero {
    grid-template-columns: 1fr;
  }

  .status-hero__secondary-actions {
    grid-template-columns: 1fr;
  }
}
```

- [ ] **Step 5: Run the stylesheet regression test**

Run from `launcher/`: `pnpm test -- tests/renderer/style-regressions.test.ts`

Expected: PASS.

- [ ] **Step 6: Commit the homepage CSS rewrite**

```bash
git add launcher/src/renderer/src/style.css
git commit -m "fix(launcher): stabilize homepage layout"
```

## Task 5: Run end-to-end renderer verification for the homepage refactor

**Files:**
- Modify: `launcher/tests/renderer/app-shell.test.tsx` if final assertion tuning is still needed
- Modify: `launcher/tests/renderer/style-regressions.test.ts` if final assertion tuning is still needed
- Verify: `launcher/src/renderer/src/AppShell.tsx`
- Verify: `launcher/src/renderer/src/style.css`

- [ ] **Step 1: Run both targeted renderer suites together**

Run from `launcher/`: `pnpm test -- tests/renderer/app-shell.test.tsx tests/renderer/style-regressions.test.ts`

Expected: PASS.

- [ ] **Step 2: Run the full launcher test suite**

Run from `launcher/`: `pnpm test`

Expected: PASS with no renderer regressions introduced by the homepage refactor.

- [ ] **Step 3: Run launcher typechecking**

Run from `launcher/`: `pnpm run typecheck`

Expected: PASS.

- [ ] **Step 4: Inspect the final diff for scope discipline**

Run: `git diff -- launcher/src/renderer/src/AppShell.tsx launcher/src/renderer/src/style.css launcher/tests/renderer/app-shell.test.tsx launcher/tests/renderer/style-regressions.test.ts`

Expected: only homepage status-layout changes, related renderer tests, and no behavior changes outside the approved scope.

- [ ] **Step 5: Commit the verification-adjusted final state**

```bash
git add launcher/src/renderer/src/AppShell.tsx launcher/src/renderer/src/style.css launcher/tests/renderer/app-shell.test.tsx launcher/tests/renderer/style-regressions.test.ts
git commit -m "fix(launcher): polish homepage status view"
```
