const { pathToFileURL } = require("node:url");

const sdkUrl = new URL("../../../sdk/nodejs/dist/index.js", pathToFileURL(__filename)).href;

(async () => {
  const { RayleaBotPlugin } = await import(sdkUrl);

  class HelloNodePlugin extends RayleaBotPlugin {
    constructor() {
      super();
      this.subscribe("message.group");
      this.onEvent("message.group", this.handleGroupMessage);
    }

    handleGroupMessage(ctx) {
      ctx.sendResult({
        handled: true,
        summary: `hello-node accepted ${ctx.eventType}`,
      });
    }
  }

  await new HelloNodePlugin().run();
})().catch((error) => {
  process.stderr.write(`${error instanceof Error ? error.stack || error.message : String(error)}\n`);
  process.exit(1);
});
