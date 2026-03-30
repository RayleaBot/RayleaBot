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
  test("parses inline YAML mappings for the server block", async () => {
    const tempRoot = await createTempDir("endpoint-inline-map");
    const configPath = path.join(tempRoot, "user.yaml");
    await fs.writeFile(configPath, 'server: { host: "::1", port: 18081 }\n', "utf8");

    const endpoint = await resolveServerEndpoint(configPath);

    expect(endpoint.host).toBe("::1");
    expect(endpoint.port).toBe(18081);
    expect(endpoint.baseUrl).toBe("http://[::1]:18081/");
  });

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

  test("reports invalid YAML before falling back to the default endpoint", async () => {
    const tempRoot = await createTempDir("endpoint-warning");
    const configPath = path.join(tempRoot, "user.yaml");
    await fs.writeFile(configPath, "server: [\n", "utf8");

    const warnings: string[] = [];
    const endpoint = await resolveServerEndpoint(configPath, {
      onWarning: ({ message }) => warnings.push(message),
    });

    expect(endpoint).toEqual({
      host: "127.0.0.1",
      port: 8080,
      baseUrl: "http://127.0.0.1:8080/",
    });
    expect(warnings).toHaveLength(1);
    expect(warnings[0]).toContain(configPath);
  });
});
