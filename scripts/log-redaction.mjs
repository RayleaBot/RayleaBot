import { StringDecoder } from "node:string_decoder";

const sensitiveKey = /access_token|authorization|cookie|proxy_url|secret|token|password|passwd|api_key/i;
const assignment = /(\b(?:setup_token|access_token|refresh_token|token|secret|password|passwd|api_key|rkey|SESSDATA|bili_jct)\s*[:=]\s*)([^&\s"'<>;,]+)/gi;
const header = /(\b(?:authorization|cookie|set-cookie)\s*[:=]\s*)([^\r\n]+)/gi;

export function redactText(value) {
  return value.replace(header, "$1[REDACTED]").replace(assignment, "$1[REDACTED]");
}

function redactValue(value) {
  if (typeof value === "string") return redactText(value);
  if (Array.isArray(value)) return value.map(redactValue);
  if (value && typeof value === "object") {
    return Object.fromEntries(Object.entries(value).map(([key, item]) => [key,
      sensitiveKey.test(key) && typeof item === "string" ? "[REDACTED]" : redactValue(item)]));
  }
  return value;
}

export function redactLogLine(line) {
  const start = line.indexOf("{");
  if (start >= 0) {
    try {
      const value = JSON.parse(line.slice(start));
      return redactText(line.slice(0, start)) + JSON.stringify(redactValue(value));
    } catch { /* Non-JSON process diagnostics use assignment/header redaction. */ }
  }
  return redactText(line);
}

// A secret or UTF-8 character may cross pipe chunks. Emit complete redacted lines.
export function createRedactedOutput(write, maxLineChars = 4 * 1024 * 1024) {
  const decoder = new StringDecoder("utf8");
  let buffered = "";
  let dropping = false;
  function accept(text) {
    for (const part of text.match(/[^\n]*\n|[^\n]+$/g) ?? []) {
      const complete = part.endsWith("\n");
      if (!dropping) {
        buffered += part;
        if (buffered.length > maxLineChars) {
          buffered = "";
          dropping = true;
          write("[诊断行超过长度上限，已省略]\n");
        }
      }
      if (complete) {
        if (!dropping) write(redactLogLine(buffered.replace(/\r?\n$/, "")) + "\n");
        buffered = "";
        dropping = false;
      }
    }
  }
  return {
    write(chunk) { accept(decoder.write(chunk)); },
    end() {
      accept(decoder.end());
      if (buffered && !dropping) write(redactLogLine(buffered));
      buffered = "";
    },
  };
}
