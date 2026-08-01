# RayleaBot Node.js SDK

`@rayleabot/sdk` is the development installation entry point for Node.js plugin authors. It pins the matching `@rayleabot/plugin-runtime` version.

Plugin packages should declare and import `@rayleabot/plugin-runtime` directly in production. RayleaBot installs those declared dependencies when the plugin is installed; built-in plugins are prepared on first start.
