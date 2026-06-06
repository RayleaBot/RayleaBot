import { readFileSync } from "node:fs";
import { describe, expect, test } from "vitest";

const styleSheetPath = new URL("../../src/renderer/src/style.css", import.meta.url);
const styleSheet = readFileSync(styleSheetPath, "utf8");

describe("renderer style regressions", () => {
  test("defines unified motion tokens and reduced-motion fallback", () => {
    expect(styleSheet).toMatch(/--motion-enter-duration:\s*220ms;/);
    expect(styleSheet).toMatch(/--motion-switch-duration:\s*180ms;/);
    expect(styleSheet).toMatch(/--motion-emphasis-duration:\s*680ms;/);
    expect(styleSheet).toMatch(/@media\s*\(prefers-reduced-motion:\s*reduce\)\s*{[\s\S]*?--motion-enter-duration:\s*1ms;/s);
    expect(styleSheet).toMatch(/@media\s*\(prefers-reduced-motion:\s*reduce\)\s*{[\s\S]*?animation-duration:\s*1ms !important;/s);
  });

  test("avoids transition all on primary interactive surfaces", () => {
    expect(styleSheet).not.toMatch(/transition:\s*all\s+/);
    expect(styleSheet).toMatch(/\.nav-item\s*{[\s\S]*?background-color var\(--motion-switch-duration\)/s);
    expect(styleSheet).toMatch(/\.check-item\s*{[\s\S]*?border-color var\(--motion-switch-duration\)/s);
    expect(styleSheet).toMatch(/\.preference-option\s*{[\s\S]*?box-shadow var\(--motion-switch-duration\)/s);
  });

  test("locks the shared section header and section transition shell", () => {
    expect(styleSheet).toMatch(/\.section-header\s*{[^}]*justify-content:\s*space-between;/s);
    expect(styleSheet).toMatch(/\.section-shell__content\s*{[^}]*transition:[^}]*opacity[^}]*transform/s);
    expect(styleSheet).toMatch(/\.section-shell\[data-transition="exiting"\]\s+\.section-shell__content\s*{[^}]*opacity:\s*0;/s);
  });

  test("locks the homepage hero layout and busy feedback", () => {
    expect(styleSheet).toMatch(/\.status-hero\s*{[^}]*grid-template-columns:\s*minmax\(0,\s*1fr\)\s*minmax\(240px,\s*320px\);/s);
    expect(styleSheet).toMatch(/\.status-hero__secondary-actions\s*{[^}]*grid-template-columns:\s*repeat\(2,\s*minmax\(0,\s*1fr\)\);/s);
    expect(styleSheet).toMatch(/\.hero-context-grid\s*{[^}]*grid-template-columns:\s*minmax\(140px,\s*180px\)\s*minmax\(0,\s*1fr\);/s);
    expect(styleSheet).toMatch(/\.status-action-feedback\[data-busy="true"\]\s+\.status-action-feedback__dot\s*{[^}]*animation:\s*busyDot/s);
  });

  test("locks the homepage responsive downgrade behavior", () => {
    expect(styleSheet).toMatch(/@media\s*\(max-width:\s*1200px\)\s*{[\s\S]*?\.status-summary-grid\s*{[\s\S]*?grid-template-columns:\s*1fr;/s);
    expect(styleSheet).toMatch(/@media\s*\(max-width:\s*960px\)\s*{[\s\S]*?\.status-hero\s*{[\s\S]*?grid-template-columns:\s*1fr;/s);
    expect(styleSheet).toMatch(/@media\s*\(max-width:\s*960px\)\s*{[\s\S]*?\.hero-context-grid[\s\S]*?grid-template-columns:\s*1fr;/s);
  });

  test("keeps environment cards readable at long content lengths", () => {
    expect(styleSheet).toMatch(/\.checks-stack--grid\s*{[^}]*grid-template-columns:\s*repeat\(auto-fit,\s*minmax\(min\(100%,\s*300px\),\s*1fr\)\)/s);
    expect(styleSheet).not.toMatch(/\.checks-stack--grid\s+\.check-item__summary,\s*\.checks-stack--grid\s+\.status-pill\s*{\s*display:\s*none !important;/s);
    expect(styleSheet).toMatch(/\.check-item__headline\s*{[^}]*justify-content:\s*space-between;/s);
    expect(styleSheet).toMatch(/\.check-item__remediation-text\s*{[^}]*overflow-wrap:\s*anywhere;/s);
    expect(styleSheet).toMatch(/\.active-environment\s+\.check-item\s*{[^}]*padding:\s*16px;[^}]*min-height:\s*0;/s);
  });

  test("keeps the diagnostics and settings comparison surfaces", () => {
    expect(styleSheet).toMatch(/\.diagnostics-context-grid\s*{[^}]*grid-template-columns:\s*repeat\(3,\s*minmax\(0,\s*1fr\)\);/s);
    expect(styleSheet).toMatch(/\.settings-compare-strip\s*{[^}]*grid-template-columns:\s*repeat\(2,\s*minmax\(0,\s*1fr\)\);/s);
    expect(styleSheet).toMatch(/\.settings-surface-tag--resolved\s*{[^}]*background:\s*var\(--accent-subtle\);/s);
  });

  test("keeps the homepage overflow safety", () => {
    expect(styleSheet).toMatch(/\.status-summary-main\s*{[^}]*min-width:\s*0;/s);
    expect(styleSheet).toMatch(/\.status-summary-rail\s*{[^}]*min-width:\s*0;/s);
    expect(styleSheet).toMatch(/\.status-log-panel\s*{[^}]*min-width:\s*0;/s);
    expect(styleSheet).toMatch(/\.status-log-surface--modern\s*{[^}]*max-height:\s*300px;[^}]*overflow-y:\s*auto;[^}]*overflow-x:\s*hidden;/s);
    expect(styleSheet).toMatch(/\.hero-context-card__value\s*{[^}]*overflow-wrap:\s*anywhere;[^}]*word-break:\s*break-word;/s);
  });
});
