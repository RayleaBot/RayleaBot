import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import fs from "node:fs";
import { createRequire } from "node:module";
import path from "node:path";
import { readPngProvenance, writePngProvenance } from "./png-provenance.mjs";
import { renderMarkSvg, validateMark } from "./design-mark.mjs";
import { compactRendering, renderLauncherMark } from "./launcher-icon-renderer.mjs";

const root = path.resolve(import.meta.dirname, "..");
const assetsRoot = path.join(root, "launcher/assets");
const manifestPath = path.join(assetsRoot, "manifest.json");
const compactSizes = [16, 20, 24, 28, 32, 40, 48, 56, 64];
const sizes = [...compactSizes, 128, 256];
const appSize = 1024;
const traySize = 32;
const trayVariants = compactSizes.filter((size) => size !== traySize).map((size) => ({ file: `tray/${size}.png`, size, mode: "light" }));
const pngAssets = [
  { file: "appicon.png", size: appSize, mode: "light", artwork: "portrait" },
  { file: "tray.png", size: traySize, mode: "light", artwork: "compact" },
  ...trayVariants.map((variant) => ({ ...variant, artwork: "compact" })),
];
const arguments_ = process.argv.slice(2);
assert(arguments_.every((argument) => argument === "--check" || argument === "--help"), "Unknown option");
if (arguments_.includes("--help")) {
  console.log("node scripts/generate-launcher-icons.mjs [--check]\nRegeneration renders the icons with the Skia-backed Canvas 2D package installed in launcher/ (@napi-rs/canvas). --check uses only Node.js.");
  process.exit(0);
}

const readText = (file) => fs.readFileSync(path.join(root, file), "utf8").replaceAll("\r\n", "\n");
const sha256 = (content) => createHash("sha256").update(content).digest("hex");
const markFiles = { portrait: "design/mark.json", compact: "design/mark-compact.json" };
const markSources = Object.fromEntries(Object.entries(markFiles).map(([artwork, file]) => [artwork, readText(file)]));
const marks = Object.fromEntries(Object.entries(markSources).map(([artwork, source]) => [artwork, validateMark(JSON.parse(source))]));
const launcherRequire = createRequire(path.join(root, "launcher/package.json"));
const canvasVersion = launcherRequire("@napi-rs/canvas/package.json").version;

const inputs = {
  geometry: Object.fromEntries(Object.entries(markFiles).map(([artwork, file]) => [artwork, { path: file, sha256: sha256(markSources[artwork]) }])),
  generator: { path: "scripts/generate-launcher-icons.mjs", sha256: sha256(readText("scripts/generate-launcher-icons.mjs")) },
  renderer: { path: "scripts/launcher-icon-renderer.mjs", sha256: sha256(readText("scripts/launcher-icon-renderer.mjs")) },
  markValidator: { path: "scripts/design-mark.mjs", sha256: sha256(readText("scripts/design-mark.mjs")) },
  metadataWriter: { path: "scripts/png-provenance.mjs", sha256: sha256(readText("scripts/png-provenance.mjs")) },
  rasterizer: { module: "@napi-rs/canvas", version: canvasVersion, engine: "Skia" },
  compactRendering,
  compactSizes,
};

const provenance = Object.fromEntries(pngAssets.map(({ file, mode, artwork, size }) => {
  const theme = marks[artwork].themes[mode];
  return [file, JSON.stringify({
    type: "source-provenance",
    method: "Deterministic Skia Canvas 2D rendering of theme-specific SVG path geometry.",
    artworkOrigin: artwork === "portrait"
      ? "User-approved light and dark character designs converted independently to pure Bezier vector paths."
      : "User-approved geometric beret badge: black rounded background, white scalloped beret, original circular bow knot and single ribbon tail; traced to pure Bezier paths.",
    geometry: inputs.geometry[artwork],
    colors: [...new Set(theme.paths.map((part) => part.fill))],
    outline: theme.outline,
    theme: mode,
    rasterizer: inputs.rasterizer,
    generator: inputs.generator,
    renderer: inputs.renderer,
    ...(artwork === "compact" ? { size, treatment: compactRendering } : {}),
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
  return Object.fromEntries([...pngAssets.map(({ file }) => file), "icon.ico"].map((file) => [file, sha256(fs.readFileSync(path.join(assetsRoot, file)))]));
}

function verify() {
  const manifest = JSON.parse(fs.readFileSync(manifestPath, "utf8"));
  assert.deepEqual(manifest.inputs, inputs, "Native icon inputs changed. Run node scripts/generate-launcher-icons.mjs");
  for (const { file, size } of pngAssets) assert.deepEqual(pngDimensions(file), { width: size, height: size });
  for (const [file, expected] of Object.entries(provenance)) {
    const embedded = readPngProvenance(fs.readFileSync(path.join(assetsRoot, file)));
    assert.equal(embedded, expected, `${file} embedded source provenance is stale`);
  }
  assert.deepEqual(icoSizes(), sizes);
  assert.equal(readText("design/mark-compact-light.svg"), renderMarkSvg(marks.compact, "light"), "Compact SVG is stale. Run node scripts/generate-design-tokens.mjs");
  assert.deepEqual(manifest.sha256, fingerprints(), "Native icon asset content differs from its generation manifest");
  console.log(`Launcher icon inputs and hashes are current: fixed light artwork, appicon ${appSize}px, tray ${compactSizes.join("/")}px, ICO ${sizes.join("/")}px.`);
}

function renderMark(canvasModule, size, mode = "light") {
  const compact = size <= 64;
  return renderLauncherMark(canvasModule, marks[compact ? "compact" : "portrait"], size, mode, compact);
}

function encodeIco(frames) {
  const directory = Buffer.alloc(6 + 16 * frames.length);
  directory.writeUInt16LE(0, 0);
  directory.writeUInt16LE(1, 2);
  directory.writeUInt16LE(frames.length, 4);
  let offset = directory.length;
  frames.forEach(({ size, png }, index) => {
    const entry = 6 + index * 16;
    directory[entry] = size >= 256 ? 0 : size;
    directory[entry + 1] = size >= 256 ? 0 : size;
    directory.writeUInt16LE(1, entry + 4);
    directory.writeUInt16LE(32, entry + 6);
    directory.writeUInt32LE(png.length, entry + 8);
    directory.writeUInt32LE(offset, entry + 12);
    offset += png.length;
  });
  return Buffer.concat([directory, ...frames.map(({ png }) => png)]);
}

if (!arguments_.includes("--check")) {
  const canvasModule = launcherRequire("@napi-rs/canvas");
  const png = (canvas) => canvas.toBuffer("image/png");
  fs.mkdirSync(path.join(assetsRoot, "tray"), { recursive: true });
  for (const { file, size, mode } of pngAssets) fs.writeFileSync(path.join(assetsRoot, file), png(renderMark(canvasModule, size, mode)));
  fs.writeFileSync(path.join(assetsRoot, "icon.ico"), encodeIco(sizes.map((size) => ({ size, png: png(renderMark(canvasModule, size)) }))));
  for (const [file, sourceProvenance] of Object.entries(provenance)) {
    const assetPath = path.join(assetsRoot, file);
    fs.writeFileSync(assetPath, writePngProvenance(fs.readFileSync(assetPath), sourceProvenance));
  }
  fs.writeFileSync(manifestPath, JSON.stringify({
    inputs,
    outputs: {
      "appicon.png": { width: appSize, height: appSize, alpha: true, role: "application", treatment: "approved light character design with opaque black-and-white fills and transparent padding" },
      "tray.png": { width: traySize, height: traySize, alpha: true, role: "system tray", treatment: "fixed black-and-white geometric beret badge shared with Web, Launcher and taskbar icons" },
      ...Object.fromEntries(trayVariants.map(({ file, size }) => [file, { width: size, height: size, alpha: true, role: "Windows system tray", treatment: "fixed light artwork rendered for the native system metric" }])),
      "icon.ico": { sizes, role: "Windows executable resource", treatment: "geometric beret badge at 16–64px shared with all compact entries; original light portrait at 128/256px" },
    },
    sha256: fingerprints(),
  }, null, 2) + "\n");
}

verify();
