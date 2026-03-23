const readline = require("node:readline");

const rl = readline.createInterface({
  input: process.stdin,
  crlfDelay: Infinity,
});

function writeFrame(frame) {
  process.stdout.write(`${JSON.stringify(frame)}\n`);
}

rl.on("line", (line) => {
  const raw = line.trim();
  if (!raw) {
    return;
  }

  const frame = JSON.parse(raw);
  const frameType = frame.type;
  const requestId = frame.request_id || "";
  const pluginId = frame.plugin_id || "hello-node";
  const timestamp = Math.floor(Date.now() / 1000);

  if (frameType === "init") {
    writeFrame({
      protocol_version: "1",
      type: "init_ack",
      timestamp,
      plugin_id: pluginId,
      request_id: requestId,
      status: "ready",
      subscriptions: ["message.group"],
    });
    return;
  }

  if (frameType === "event") {
    writeFrame({
      protocol_version: "1",
      type: "result",
      timestamp,
      plugin_id: pluginId,
      request_id: requestId,
      status: "success",
      data: {
        handled: true,
        summary: `hello-node accepted ${frame.event?.event_type ?? "unknown"}`,
      },
    });
    return;
  }

  // This example intentionally keeps the protocol surface narrow.
  // shutdown, error handling, and other message types stay outside this
  // minimal sample.
});
