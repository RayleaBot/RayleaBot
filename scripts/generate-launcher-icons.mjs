import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import fs from "node:fs";
import { createRequire } from "node:module";
import path from "node:path";
import { readPngProvenance, writePngProvenance } from "./png-provenance.mjs";
import { validateMark } from "./design-mark.mjs";

const root = path.resolve(import.meta.dirname, "..");
const assetsRoot = path.join(root, "launcher/assets");
const manifestPath = path.join(assetsRoot, "manifest.json");
const sizes = [16, 24, 32, 48, 64, 128, 256];
const appSize = 1024;
const traySize = 32;
const arguments_ = process.argv.slice(2);
assert(arguments_.every((argument) => argument === "--check" || argument === "--help"), "Unknown option");
if (arguments_.includes("--help")) {
  console.log("node scripts/generate-launcher-icons.mjs [--check]\nRegeneration renders the icons with the Skia-backed Canvas 2D package installed in launcher/ (@napi-rs/canvas). --check uses only Node.js.");
  process.exit(0);
}

const readText = (file) => fs.readFileSync(path.join(root, file), "utf8").replaceAll("\r\n", "\n");
const sha256 = (content) => createHash("sha256").update(content).digest("hex");
const markSource = readText("design/mark.json");
const mark = validateMark(JSON.parse(markSource));
const viewBox = mark.viewBox.trim().split(/[\s,]+/).map(Number);
assert(viewBox.length === 4 && viewBox.every(Number.isFinite) && viewBox[2] > 0 && viewBox[3] > 0, "Invalid mark viewBox");
const launcherRequire = createRequire(path.join(root, "launcher/package.json"));
const canvasVersion = launcherRequire("@napi-rs/canvas/package.json").version;

const inputs = {
  geometry: { path: "design/mark.json", sha256: sha256(markSource) },
  generator: { path: "scripts/generate-launcher-icons.mjs", sha256: sha256(readText("scripts/generate-launcher-icons.mjs")) },
  markValidator: { path: "scripts/design-mark.mjs", sha256: sha256(readText("scripts/design-mark.mjs")) },
  metadataWriter: { path: "scripts/png-provenance.mjs", sha256: sha256(readText("scripts/png-provenance.mjs")) },
  rasterizer: { module: "@napi-rs/canvas", version: canvasVersion, engine: "Skia" },
};

const provenance = Object.fromEntries(["appicon.png", "tray.png", "tray-dark.png"].map((file) => {
  const mode = file === "tray-dark.png" ? "dark" : "light";
  const theme = mark.themes[mode];
  return [file, JSON.stringify({
    type: "source-provenance",
    method: "Deterministic Skia Canvas 2D rendering of theme-specific SVG path geometry.",
    artworkOrigin: "User-approved light and dark character designs converted independently to pure Bezier vector paths.",
    geometry: inputs.geometry,
    colors: [...new Set(theme.paths.map((part) => part.fill))],
    outline: theme.outline,
    theme: mode,
    rasterizer: inputs.rasterizer,
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
  return Object.fromEntries(["appicon.png", "tray.png", "tray-dark.png", "icon.ico"].map((file) => [file, sha256(fs.readFileSync(path.join(assetsRoot, file)))]));
}

function verify() {
  const manifest = JSON.parse(fs.readFileSync(manifestPath, "utf8"));
  assert.deepEqual(manifest.inputs, inputs, "Native icon inputs changed. Run node scripts/generate-launcher-icons.mjs");
  assert.deepEqual(pngDimensions("appicon.png"), { width: appSize, height: appSize });
  assert.deepEqual(pngDimensions("tray.png"), { width: traySize, height: traySize });
  assert.deepEqual(pngDimensions("tray-dark.png"), { width: traySize, height: traySize });
  for (const [file, expected] of Object.entries(provenance)) {
    const embedded = readPngProvenance(fs.readFileSync(path.join(assetsRoot, file)));
    assert.equal(embedded, expected, `${file} embedded source provenance is stale`);
  }
  assert.deepEqual(icoSizes(), sizes);
  assert.deepEqual(manifest.sha256, fingerprints(), "Native icon asset content differs from its generation manifest");
  console.log(`Launcher icon inputs and hashes are current: appicon ${appSize}px, light/dark tray ${traySize}px, ICO ${sizes.join("/")}px.`);
}

function renderMark(canvasModule, size, mode = "light") {
  const { createCanvas, Path2D } = canvasModule;
  const theme = mark.themes[mode];
  const canvas = createCanvas(size, size);
  const ctx = canvas.getContext("2d");
  const scale = size / Math.max(viewBox[2], viewBox[3]);
  ctx.translate((size - viewBox[2] * scale) / 2 - viewBox[0] * scale, (size - viewBox[3] * scale) / 2 - viewBox[1] * scale);
  ctx.scale(scale, scale);
  ctx.strokeStyle = theme.outline.color;
  ctx.lineWidth = theme.outline.width;
  ctx.lineJoin = "round";
  ctx.stroke(new Path2D(theme.paths[theme.outline.pathIndex].d));
  for (const part of theme.paths) {
    ctx.fillStyle = part.fill;
    ctx.globalAlpha = part.opacity;
    ctx.fill(new Path2D(part.d), part.fillRule);
  }
  return canvas;
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
  fs.writeFileSync(path.join(assetsRoot, "appicon.png"), png(renderMark(canvasModule, appSize)));
  fs.writeFileSync(path.join(assetsRoot, "tray.png"), png(renderMark(canvasModule, traySize)));
  fs.writeFileSync(path.join(assetsRoot, "tray-dark.png"), png(renderMark(canvasModule, traySize, "dark")));
  fs.writeFileSync(path.join(assetsRoot, "icon.ico"), encodeIco(sizes.map((size) => ({ size, png: png(renderMark(canvasModule, size)) }))));
  for (const [file, sourceProvenance] of Object.entries(provenance)) {
    const assetPath = path.join(assetsRoot, file);
    fs.writeFileSync(assetPath, writePngProvenance(fs.readFileSync(assetPath), sourceProvenance));
  }
  fs.writeFileSync(manifestPath, JSON.stringify({
    inputs,
    outputs: {
      "appicon.png": { width: appSize, height: appSize, alpha: true, role: "application", treatment: "approved light character design with opaque black-and-white fills and transparent padding" },
      "tray.png": { width: traySize, height: traySize, alpha: true, role: "light system tray", treatment: "light character design with a fine light outline and transparent background" },
      "tray-dark.png": { width: traySize, height: traySize, alpha: true, role: "dark system tray", treatment: "independent dark character design with white hair, charcoal accessories, gray clothing and transparent background" },
      "icon.ico": { sizes, role: "Windows executable resource", treatment: "application composition rendered natively at every frame size" },
    },
    sha256: fingerprints(),
  }, null, 2) + "\n");
}

verify();
