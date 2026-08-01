# RayleaBot Node.js plugin runtime

`@rayleabot/plugin-runtime` provides the production client used by Node.js plugins to communicate with RayleaBot.

Plugins should declare an exact runtime dependency in `info.json` and import the client directly:

```json
{
  "dependencies": {
    "nodejs": ["@rayleabot/plugin-runtime@0.1.0"]
  }
}
```

```js
import { RayleaBotPlugin } from "@rayleabot/plugin-runtime";
```
