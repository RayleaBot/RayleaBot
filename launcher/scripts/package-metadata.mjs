export function resolveMacBundleVersion({ releaseTag = "", packageVersion = "" } = {}) {
  for (const candidate of [releaseTag, packageVersion]) {
    const match = String(candidate).trim().match(/^v?(\d+\.\d+\.\d+)(?:[-+][0-9A-Za-z.-]+)?$/);
    if (match) {
      return match[1];
    }
  }
  throw new Error("Launcher version must be a semantic version");
}
