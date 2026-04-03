import { readFileSync } from "node:fs";
import { describe, expect, test } from "vitest";

const styleSheetPath = new URL("../../src/renderer/src/style.css", import.meta.url);
const styleSheet = readFileSync(styleSheetPath, "utf8");

describe("renderer style regressions", () => {
  test("keeps a shadow-safe gutter around the environment metric summary", () => {
    expect(styleSheet).toMatch(/\.metric-panel-container\s*{[^}]*overflow:\s*visible;/s);
    expect(styleSheet).not.toMatch(/\.metric-panel-container\s*{[^}]*margin-inline:\s*-4px;/s);
  });

  test("keeps the settings lower cards responsive instead of hard-pinning two columns", () => {
    expect(styleSheet).toMatch(/\.settings-lower-grid\s*{[^}]*grid-template-columns:\s*repeat\(auto-fit,\s*minmax\(min\(100%,\s*320px\),\s*1fr\)\);/s);
    expect(styleSheet).toMatch(/\.preferences-panel,\s*\.maintenance-panel\s*{[^}]*min-width:\s*0;/s);
    expect(styleSheet).not.toMatch(/\.settings-lower-grid\s*{[^}]*grid-template-columns:\s*minmax\(0,\s*1\.2fr\)\s*minmax\(320px,\s*0\.8fr\);/s);
  });
});
