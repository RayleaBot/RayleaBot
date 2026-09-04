import test from "node:test";
import assert from "node:assert/strict";
import { createRedactedOutput, redactLogLine } from "../log-redaction.mjs";

test("pipe chunk boundaries cannot reveal setup credentials or corrupt Chinese", () => {
  for (let size = 1; size < 30; size++) {
    let output = "";
    const sink = createRedactedOutput((chunk) => { output += chunk; });
    const input = Buffer.from("首次设置地址：http://localhost/setup#setup_token=fixture-only-token\n消息正常\n");
    for (let i = 0; i < input.length; i += size) sink.write(input.subarray(i, i + size));
    sink.end();
    assert.ok(!output.includes("fixture-only-token"));
    assert.ok(output.includes("首次设置地址"));
    assert.ok(output.includes("消息正常"));
  }
});

test("structured secrets are masked while diagnostic identity is retained", () => {
  const line = redactLogLine(JSON.stringify({ ts: "fixture", request_id: "req-1", details: { cookie: "fixture-cookie", code: "adapter.send_unconfirmed" } }));
  const result = JSON.parse(line);
  assert.equal(result.request_id, "req-1");
  assert.equal(result.details.code, "adapter.send_unconfirmed");
  assert.equal(result.details.cookie, "[REDACTED]");
});

test("JSON diagnostics also redact credentials in their text prefix", () => {
  const line = redactLogLine('setup_token=fixture-prefix {"request_id":"req-1","details":{"token":"fixture-json"}}');
  assert.ok(!line.includes("fixture-prefix"));
  assert.ok(!line.includes("fixture-json"));
  assert.ok(line.includes("req-1"));
});

test("oversized lines are bounded and never emit their secret fragments", () => {
  let output = "";
  const sink = createRedactedOutput((chunk) => { output += chunk; }, 16);
  sink.write(Buffer.from("setup_token=fixture-only-"));
  sink.write(Buffer.from("credential\nnormal\n"));
  sink.end();
  assert.ok(!output.includes("fixture-only"));
  assert.ok(output.includes("normal"));
});
