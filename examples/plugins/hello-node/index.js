(async () => {
  const { RayleaBotPlugin } = await import("@rayleabot/plugin-runtime");

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
