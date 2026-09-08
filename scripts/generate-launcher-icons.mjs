import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import fs from "node:fs";
import { createRequire } from "node:module";
import path from "node:path";
import { runGo } from "../launcher/scripts/run-go.mjs";
import { readPngProvenance, writePngProvenance } from "./png-provenance.mjs";

const root = path.resolve(import.meta.dirname, "..");
const assetsRoot = path.join(root, "launcher/assets");
const manifestPath = path.join(assetsRoot, "manifest.json");
const sizes = [16, 24, 32, 48, 64, 128, 256];
const arguments_ = process.argv.slice(2);
assert(arguments_.every((argument) => argument === "--check" || argument === "--help"), "Unknown option");
if (arguments_.includes("--help")) {
  console.log("node scripts/generate-launcher-icons.mjs [--check]\nRegeneration uses the installed Web Playwright Chromium and the Wails version in launcher/go.mod. --check uses only Node.js.");
  process.exit(0);
}

const readText = (file) => fs.readFileSync(path.join(root, file), "utf8").replaceAll("\r\n", "\n");
const sha256 = (content) => createHash("sha256").update(content).digest("hex");
const markSource = readText("design/mark.json");
const mark = JSON.parse(markSource);
const tokens = JSON.parse(readText("design/tokens.json"));
const wailsVersion = readText("launcher/go.mod").match(/^\s*github\.com\/wailsapp\/wails\/v3\s+(v\S+)\s*$/m)?.[1];
assert(wailsVersion, "launcher/go.mod must declare the Wails version");
assert(typeof mark.viewBox === "string" && Array.isArray(mark.paths) && mark.paths.length > 0, "Invalid design/mark.json");

function color(role) {
  const value = role.split(".").reduce((node, key) => node[key], tokens).$value;
  if (typeof value === "string" && value.startsWith("{")) return color(value.slice(1, -1));
  assert(/^#[0-9a-f]{6}$/i.test(value.hex), `Invalid color role ${role}`);
  return value.hex;
}

const colors = Object.fromEntries([
  "semantic.light.color.brandFill",
  "semantic.light.color.surfaceSoft",
  "base.color.celadon.600",
].map((role) => [role, color(role)]));
const inputs = {
  geometry: { path: "design/mark.json", sha256: sha256(markSource) },
  palette: { path: "design/tokens.json", roles: colors, sha256: sha256(JSON.stringify(colors)) },
  generator: { path: "scripts/generate-launcher-icons.mjs", sha256: sha256(readText("scripts/generate-launcher-icons.mjs")) },
  metadataWriter: { path: "scripts/png-provenance.mjs", sha256: sha256(readText("scripts/png-provenance.mjs")) },
  icoEncoder: { module: "github.com/wailsapp/wails/v3", version: wailsVersion },
};

function paths(fill, silhouette = false) {
  return mark.paths.map((part) => `    <path d="${part.d}" fill="${fill}" opacity="${silhouette ? 1 : part.opacity}"/>`).join("\n");
}

const sources = {
  "sources/appicon.svg": `<svg xmlns="http://www.w3.org/2000/svg" width="1024" height="1024" viewBox="0 0 32 32">
  <rect x="1.5" y="1.5" width="29" height="29" rx="6.5" fill="${colors["semantic.light.color.surfaceSoft"]}"/>
  <svg x="1.6" y="0.4" width="28.8" height="31.2" viewBox="${mark.viewBox}">
${paths(colors["semantic.light.color.brandFill"])}
  </svg>
</svg>
`,
  "sources/tray.svg": `<svg xmlns="http://www.w3.org/2000/svg" width="32" height="32" viewBox="${mark.viewBox}">
${paths(colors["base.color.celadon.600"], true)}
</svg>
`,
};

const provenance = Object.fromEntries([
  ["appicon.png", ["semantic.light.color.brandFill", "semantic.light.color.surfaceSoft"]],
  ["tray.png", ["base.color.celadon.600"]],
].map(([file, roles]) => {
  const vector = `sources/${path.basename(file, ".png")}.svg`;
  return [file, JSON.stringify({
    type: "source-provenance",
    method: "Deterministic SVG rasterization of an approved repository mark; no AI image generation.",
    geometry: inputs.geometry,
    palette: { path: inputs.palette.path, roles: Object.fromEntries(roles.map((role) => [role, colors[role]])) },
    vector: { path: `launcher/assets/${vector}`, sha256: sha256(sources[vector]) },
    generator: inputs.generator,
    metadataWriter: inputs.metadataWriter,
  }, null, 2)];
}));

function pngDimensions(file) {
  const bytes = fs.readFileSync(path.join(assetsRoot, file));
  assert(bytes.subarray(0, 8).equals(Buffer.from([137, 80, 78, 71, 13, 10, 26, 10])), `${file} is not PNG`);
  assert.equal(bytes.toString("ascii", 12, 16), "IHDR", `${file} has no PNG header`);
  assert.equal(bytes[25], 6, `${file} must preserve RGBA transparency`);
  return { width: bytes.readUInt32BE(16), height: bytes.readUInt32BE(20) };
}

function icoSizes() {
  const bytes = fs.readFileSync(path.join(assetsRoot, "icon.ico"));
  assert.equal(bytes.readUInt16LE(0), 0, "Invalid ICO header");
  assert.equal(bytes.readUInt16LE(2), 1, "icon.ico is not an icon");
  const count = bytes.readUInt16LE(4);
  assert.equal(count, sizes.length, "Missing ICO image entries");
  return Array.from({ length: count }, (_, index) => {
    const entry = 6 + index * 16;
    const width = bytes[entry] || 256;
    const height = bytes[entry + 1] || 256;
    assert.equal(width, height, "ICO image must be square");
    const length = bytes.readUInt32LE(entry + 8);
    const offset = bytes.readUInt32LE(entry + 12);
    assert(length > 0 && offset >= 6 + count * 16 && offset + length <= bytes.length, "Invalid ICO image data");
    return width;
  }).sort((left, right) => left - right);
}

function fingerprints() {
  return Object.fromEntries(["appicon.png", "tray.png", "icon.ico", ...Object.keys(sources)].map((file) => {
    const bytes = fs.readFileSync(path.join(assetsRoot, file));
    return [file, sha256(file.endsWith(".svg") ? bytes.toString("utf8").replaceAll("\r\n", "\n") : bytes)];
  }));
}

function verify() {
  const manifest = JSON.parse(fs.readFileSync(manifestPath, "utf8"));
  const regenerate = "Native icon sources changed. Run node scripts/generate-launcher-icons.mjs";
  assert.deepEqual(manifest.inputs, inputs, regenerate);
  for (const [file, expected] of Object.entries(sources)) {
    assert.equal(fs.readFileSync(path.join(assetsRoot, file), "utf8").replaceAll("\r\n", "\n"), expected, `${file} is stale`);
  }
  assert.deepEqual(pngDimensions("appicon.png"), { width: 1024, height: 1024 });
  assert.deepEqual(pngDimensions("tray.png"), { width: 32, height: 32 });
  for (const [file, expected] of Object.entries(provenance)) {
    const embedded = readPngProvenance(fs.readFileSync(path.join(assetsRoot, file)));
    assert.equal(embedded, expected, `${file} embedded source provenance is stale`);
  }
  assert.deepEqual(icoSizes(), sizes);
  assert.deepEqual(manifest.sha256, fingerprints(), "Native icon asset content differs from its generation manifest");
  console.log("Launcher icon sources and hashes are current: appicon 1024px, tray 32px, ICO 16/24/32/48/64/128/256px.");
}

if (!arguments_.includes("--check")) {
  const webRequire = createRequire(path.join(root, "web/package.json"));
  const playwrightRequire = createRequire(webRequire.resolve("@playwright/test/package.json"));
  const { chromium } = playwrightRequire("playwright");
  const browser = await chromium.launch({ headless: true });
  let browserVersion;
  try {
    browserVersion = browser.version();
    fs.mkdirSync(path.join(assetsRoot, "sources"), { recursive: true });
    const page = await browser.newPage({ deviceScaleFactor: 1 });
    for (const [file, svg] of Object.entries(sources)) {
      fs.writeFileSync(path.join(assetsRoot, file), svg);
      const size = file.endsWith("appicon.svg") ? 1024 : 32;
      await page.setViewportSize({ width: size, height: size });
      await page.setContent(`<html><head><style>html,body{margin:0;background:transparent}svg{display:block}</style></head><body>${svg}</body></html>`);
      await page.screenshot({ path: path.join(assetsRoot, path.basename(file, ".svg") + ".png"), omitBackground: true });
    }
  } finally {
    await browser.close();
  }
  const exitCode = await runGo([
    "run", `github.com/wailsapp/wails/v3/cmd/wails3@${wailsVersion}`,
    "generate", "icons", "-input", path.join(assetsRoot, "appicon.png"),
    "-windowsfilename", path.join(assetsRoot, "icon.ico"), "-macfilename=",
    "-sizes", sizes.join(","),
  ]);
  assert.equal(exitCode, 0, "Wails icon generation failed");
  for (const [file, sourceProvenance] of Object.entries(provenance)) {
    const assetPath = path.join(assetsRoot, file);
    fs.writeFileSync(assetPath, writePngProvenance(fs.readFileSync(assetPath), sourceProvenance));
  }
  fs.writeFileSync(manifestPath, JSON.stringify({
    inputs,
    rasterizer: { playwright: playwrightRequire("playwright/package.json").version, chromium: browserVersion },
    outputs: {
      "appicon.png": { width: 1024, height: 1024, alpha: true, role: "application", treatment: "original facets on a neutral tile with transparent padding" },
      "tray.png": { width: 32, height: 32, alpha: true, role: "system tray", treatment: "original geometry as an opaque silhouette" },
      "icon.ico": { sizes, role: "Windows executable resource" },
    },
    sha256: fingerprints(),
  }, null, 2) + "\n");
}

verify();
