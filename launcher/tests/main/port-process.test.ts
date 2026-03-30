import { describe, expect, test, vi } from "vitest";
import { tryStopEndpointProcess } from "@main/services/port-process";

const endpoint = {
  host: "127.0.0.1",
  port: 8080,
  baseUrl: "http://127.0.0.1:8080/",
};

describe("tryStopEndpointProcess", () => {
  test("does not terminate unrelated Windows processes that happen to own the port", async () => {
    const execFileAsync = vi.fn(async (file: string, args: string[]) => {
      if (file === "netstat") {
        expect(args).toEqual(["-ano", "-p", "tcp"]);
        return {
          stdout: "  TCP    127.0.0.1:8080    0.0.0.0:0    LISTENING    4321\r\n",
          stderr: "",
        };
      }

      if (file === "tasklist") {
        expect(args).toEqual(["/FI", "PID eq 4321", "/FO", "CSV", "/NH"]);
        return {
          stdout: '"python.exe","4321","Console","1","10,000 K"\r\n',
          stderr: "",
        };
      }

      throw new Error(`unexpected command: ${file}`);
    });
    const terminateProcessId = vi.fn(async () => true);

    const stopped = await tryStopEndpointProcess(endpoint, {
      platform: "win32",
      execFileAsync,
      terminateProcessId,
    });

    expect(stopped).toBe(false);
    expect(terminateProcessId).not.toHaveBeenCalled();
  });

  test("terminates raylea-server when it owns the listening port", async () => {
    const execFileAsync = vi.fn(async (file: string) => {
      if (file === "netstat") {
        return {
          stdout: "  TCP    127.0.0.1:8080    0.0.0.0:0    LISTENING    4242\r\n",
          stderr: "",
        };
      }

      if (file === "tasklist") {
        return {
          stdout: '"raylea-server.exe","4242","Console","1","10,000 K"\r\n',
          stderr: "",
        };
      }

      throw new Error(`unexpected command: ${file}`);
    });
    const terminateProcessId = vi.fn(async () => true);

    const stopped = await tryStopEndpointProcess(endpoint, {
      platform: "win32",
      execFileAsync,
      terminateProcessId,
    });

    expect(stopped).toBe(true);
    expect(terminateProcessId).toHaveBeenCalledWith(4242);
  });
});
