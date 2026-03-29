import fs from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import { afterEach, describe, expect, test } from "vitest";
import { resolveServerEndpoint } from "@main/services/endpoint-resolver";

const tempRoots: string[] = [];

async function createTempDir(label: string) {
  const tempRoot = await fs.mkdtemp(path.join(os.tmpdir(), `raylea-${label}-`));
  tempRoots.push(tempRoot);
  return tempRoot;
}

afterEach(async () => {
  await Promise.all(
    tempRoots.splice(0).map(async (target) => {
      await fs.rm(target, { recursive: true, force: true });
    }),
  );
});

describe("resolveServerEndpoint", () => {
  test("wraps IPv6 hosts in brackets when building the base url", async () => {
    const tempRoot = await createTempDir("endpoint-ipv6");
    const configPath = path.join(tempRoot, "user.yaml");
    await fs.writeFile(
      configPath,
      ["server:", "  host: ::1", "  port: 18080"].join("\n"),
      "utf8",
    );

    const endpoint = await resolveServerEndpoint(configPath);

    expect(endpoint.host).toBe("::1");
    expect(endpoint.baseUrl).toBe("http://[::1]:18080/");
  });
});
