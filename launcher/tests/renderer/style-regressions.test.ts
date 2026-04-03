import { readFileSync } from "node:fs";
import { describe, expect, test } from "vitest";

const styleSheetPath = new URL("../../src/renderer/src/style.css", import.meta.url);
const styleSheet = readFileSync(styleSheetPath, "utf8");

describe("renderer style regressions", () => {
  test("keeps a shadow-safe gutter around the environment metric summary", () => {
    expect(styleSheet).toMatch(/\.metric-panel-container\s*{[^}]*overflow:\s*visible;/s);
    expect(styleSheet).not.toMatch(/\.metric-panel-container\s*{[^}]*margin-inline:\s*-4px;/s);
    expect(styleSheet).toMatch(/\.metric-panel::before\s*{[^}]*radial-gradient/s);
    expect(styleSheet).not.toMatch(/\.metric-panel\s*{[^}]*box-shadow:\s*0 -18px 38px rgba\(0,\s*0,\s*0,\s*0\.56\), 0 10px 24px rgba\(0,\s*0,\s*0,\s*0\.22\);/s);
  });

  test("keeps environment check cards inside the scroll area without clipped outer shadows", () => {
    expect(styleSheet).toMatch(/\.active-environment \.checks-stack--grid\s*{[^}]*padding:\s*2px 8px 6px;/s);
    expect(styleSheet).toMatch(/\.active-environment \.check-item\s*{[^}]*box-shadow:\s*inset 0 1px 0/s);
    expect(styleSheet).toMatch(/\.active-environment \.check-item:hover\s*{[^}]*transform:\s*none;/s);
    expect(styleSheet).not.toMatch(/\.active-environment \.check-item\s*{[^}]*box-shadow:\s*0 2px 8px rgba\(0,\s*0,\s*0,\s*0\.1\);/s);
    expect(styleSheet).not.toMatch(/\.active-environment \.check-item:hover\s*{[^}]*box-shadow:\s*0 4px 12px rgba\(0,\s*0,\s*0,\s*0\.2\);/s);
  });

  test("keeps settings cards responsive after merging advanced overrides with resolved paths", () => {
    expect(styleSheet).toMatch(/\.settings-layout\s*{[^}]*grid-template-columns:\s*minmax\(0,\s*1fr\)\s*minmax\(300px,\s*360px\);/s);
    expect(styleSheet).toMatch(/@media\s*\(max-width:\s*1100px\)\s*{[^}]*\.settings-layout\s*{[^}]*grid-template-columns:\s*1fr;/s);
    expect(styleSheet).toMatch(/\.settings-resolution-panel\s*{[^}]*border-top:\s*1px solid/s);
    expect(styleSheet).not.toMatch(/\.settings-column--secondary\s*{[^}]*position:\s*sticky;/s);
  });

  test("uses an edit rail and action cards for the highlighted settings controls", () => {
    expect(styleSheet).toMatch(/\.settings-edit-bar\s*{[^}]*justify-content:\s*space-between;/s);
    expect(styleSheet).toMatch(/\.maintenance-action-card\s*{[^}]*justify-content:\s*space-between;/s);
    expect(styleSheet).toMatch(/\.maintenance-action-card--danger\s*{[^}]*box-shadow:\s*inset 3px 0 0/s);
  });

  test("locks the homepage hero layout", () => {
    expect(styleSheet).toMatch(/\.status-hero\s*{[^}]*grid-template-columns:\s*minmax\(0,\s*1fr\)\s*minmax\(240px,\s*320px\);/s);
    expect(styleSheet).toMatch(/\.status-hero__secondary-actions\s*{[^}]*grid-template-columns:\s*repeat\(2,\s*minmax\(0,\s*1fr\)\);/s);
  });

  test("locks the homepage responsive downgrade behavior", () => {
    expect(styleSheet).toMatch(/@media\s*\(max-width:\s*1200px\)\s*{[\s\S]*?\.status-summary-grid\s*{[\s\S]*?grid-template-columns:\s*1fr;/s);
    expect(styleSheet).toMatch(/@media\s*\(max-width:\s*960px\)\s*{[\s\S]*?\.status-hero\s*{[\s\S]*?grid-template-columns:\s*1fr;/s);
    expect(styleSheet).toMatch(/@media\s*\(max-width:\s*960px\)\s*{[\s\S]*?\.status-hero__secondary-actions\s*{[\s\S]*?grid-template-columns:\s*1fr;/s);
  });

  test("locks the homepage overflow safety", () => {
    expect(styleSheet).toMatch(/\.status-summary-main\s*{[^}]*min-width:\s*0;/s);
    expect(styleSheet).toMatch(/\.status-summary-rail\s*{[^}]*min-width:\s*0;/s);
    expect(styleSheet).toMatch(/\.status-log-panel\s*{[^}]*min-width:\s*0;/s);
    expect(styleSheet).toMatch(/\.status-log-surface--modern\s*{[^}]*max-height:\s*300px;[^}]*overflow-y:\s*auto;[^}]*overflow-x:\s*hidden;/s);
  });
});
