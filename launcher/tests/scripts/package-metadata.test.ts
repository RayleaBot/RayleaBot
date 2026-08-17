import { describe, expect, test } from "vitest";

import { resolveMacBundleVersion } from "../../scripts/package-metadata.mjs";

describe("launcher package metadata", () => {
  test("uses the release tag for the macOS bundle version", () => {
    expect(resolveMacBundleVersion({ releaseTag: "v1.2.3", packageVersion: "0.1.0" })).toBe("1.2.3");
  });

  test("falls back to the package version outside release builds", () => {
    expect(resolveMacBundleVersion({ packageVersion: "0.4.0-beta.2" })).toBe("0.4.0");
  });
});
