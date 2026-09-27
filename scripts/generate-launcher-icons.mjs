import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import fs from "node:fs";
import { createRequire } from "node:module";
import path from "node:path";
import { readPngProvenance, writePngProvenance } from "./png-provenance.mjs";

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
const mark = JSON.parse(markSource);
const tokens = JSON.parse(readText("design/tokens.json"));
assert(typeof mark.viewBox === "string" && Array.isArray(mark.paths) && mark.paths.length > 0, "Invalid design/mark.json");
const viewBox = mark.viewBox.trim().split(/[\s,]+/).map(Number);
assert(viewBox.length === 4 && viewBox.every(Number.isFinite) && viewBox[2] > 0 && viewBox[3] > 0, "Invalid mark viewBox");
const launcherRequire = createRequire(path.join(root, "launcher/package.json"));
const canvasVersion = launcherRequire("@napi-rs/canvas/package.json").version;

function color(role) {
  const value = role.split(".").reduce((node, key) => node[key], tokens).$value;
  if (typeof value === "string" && value.startsWith("{")) return color(value.slice(1, -1));
  assert(/^#[0-9a-f]{6}$/i.test(value.hex), `Invalid color role ${role}`);
  return value.hex;
}

// Palette roles consumed by the composition. The tile runs from a lighter to a darker celadon
// around the brand fill; the mark is drawn in on-brand tones; the tray uses the mid celadon.
const roles = {
  tileTop: "base.color.celadon.600",
  tileBottom: "base.color.celadon.800",
  ink: "semantic.light.color.onBrand",
  inkShade: "base.color.celadon.100",
  tray: "base.color.celadon.600",
};
const colors = Object.fromEntries(Object.values(roles).map((role) => [role, color(role)]));
const paint = Object.fromEntries(Object.entries(roles).map(([key, role]) => [key, colors[role]]));
const inputs = {
  geometry: { path: "design/mark.json", sha256: sha256(markSource) },
  palette: { path: "design/tokens.json", roles: colors, sha256: sha256(JSON.stringify(colors)) },
  generator: { path: "scripts/generate-launcher-icons.mjs", sha256: sha256(readText("scripts/generate-launcher-icons.mjs")) },
  metadataWriter: { path: "scripts/png-provenance.mjs", sha256: sha256(readText("scripts/png-provenance.mjs")) },
  rasterizer: { module: "@napi-rs/canvas", version: canvasVersion, engine: "Skia" },
};

const provenance = Object.fromEntries([
  ["appicon.png", ["tileTop", "tileBottom", "ink", "inkShade"]],
  ["tray.png", ["tray"]],
].map(([file, keys]) => [file, JSON.stringify({
  type: "source-provenance",
  method: "Deterministic Skia Canvas 2D rendering of an approved repository mark; no AI image generation.",
  geometry: inputs.geometry,
  palette: { path: inputs.palette.path, roles: Object.fromEntries(keys.map((key) => [roles[key], colors[roles[key]]])) },
  rasterizer: inputs.rasterizer,
  generator: inputs.generator,
  metadataWriter: inputs.metadataWriter,
}, null, 2)]));

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
  return Object.fromEntries(["appicon.png", "tray.png", "icon.ico"].map((file) => [file, sha256(fs.readFileSync(path.join(assetsRoot, file)))]));
}

function verify() {
  const manifest = JSON.parse(fs.readFileSync(manifestPath, "utf8"));
  assert.deepEqual(manifest.inputs, inputs, "Native icon inputs changed. Run node scripts/generate-launcher-icons.mjs");
  assert.deepEqual(pngDimensions("appicon.png"), { width: appSize, height: appSize });
  assert.deepEqual(pngDimensions("tray.png"), { width: traySize, height: traySize });
  for (const [file, expected] of Object.entries(provenance)) {
    const embedded = readPngProvenance(fs.readFileSync(path.join(assetsRoot, file)));
    assert.equal(embedded, expected, `${file} embedded source provenance is stale`);
  }
  assert.deepEqual(icoSizes(), sizes);
  assert.deepEqual(manifest.sha256, fingerprints(), "Native icon asset content differs from its generation manifest");
  console.log(`Launcher icon inputs and hashes are current: appicon ${appSize}px, tray ${traySize}px, ICO ${sizes.join("/")}px.`);
}

// Composition in 32 design units per canvas side, so every output size shares one layout.
const units = 32;
const tileInset = 1.5;
const tileRadius = 7;
const markBox = 21;
const trayMarkBox = 28;

function withMark(ctx, size, box, draw) {
  const scale = (box / Math.max(viewBox[2], viewBox[3])) * (size / units);
  ctx.save();
  ctx.translate((size - viewBox[2] * scale) / 2 - viewBox[0] * scale, (size - viewBox[3] * scale) / 2 - viewBox[1] * scale);
  ctx.scale(scale, scale);
  draw();
  ctx.restore();
}

function renderApplication(canvasModule, size) {
  const { createCanvas, Path2D } = canvasModule;
  const canvas = createCanvas(size, size);
  const ctx = canvas.getContext("2d");
  const unit = size / units;
  const inset = tileInset * unit;
  const side = size - 2 * inset;
  const tile = new Path2D();
  tile.roundRect(inset, inset, side, side, tileRadius * unit);
  const paths = mark.paths.map((part) => ({ path: new Path2D(part.d), opacity: part.opacity }));

  const base = ctx.createLinearGradient(0, inset, 0, inset + side);
  base.addColorStop(0, paint.tileTop);
  base.addColorStop(1, paint.tileBottom);
  ctx.fillStyle = base;
  ctx.fill(tile);

  ctx.save();
  ctx.clip(tile);
  // Glaze: a soft light from the upper left and a darker foot give the tile porcelain depth.
  const sheen = ctx.createRadialGradient(size * 0.3, size * 0.12, 0, size * 0.3, size * 0.12, size * 0.85);
  sheen.addColorStop(0, "rgba(255,255,255,0.22)");
  sheen.addColorStop(0.55, "rgba(255,255,255,0.05)");
  sheen.addColorStop(1, "rgba(255,255,255,0)");
  ctx.fillStyle = sheen;
  ctx.fillRect(0, 0, size, size);
  const foot = ctx.createLinearGradient(0, inset + side * 0.55, 0, inset + side);
  foot.addColorStop(0, "rgba(0,0,0,0)");
  foot.addColorStop(1, "rgba(0,0,0,0.16)");
  ctx.fillStyle = foot;
  ctx.fillRect(0, 0, size, size);
  // Inner rim: light along the top edge, shade along the bottom edge.
  const rim = ctx.createLinearGradient(0, inset, 0, inset + side);
  rim.addColorStop(0, "rgba(255,255,255,0.42)");
  rim.addColorStop(0.35, "rgba(255,255,255,0.07)");
  rim.addColorStop(0.75, "rgba(0,0,0,0.05)");
  rim.addColorStop(1, "rgba(0,0,0,0.2)");
  ctx.strokeStyle = rim;
  ctx.lineWidth = Math.max(1.3 * unit, 1);
  ctx.stroke(tile);

  // Soft shadow lifts the mark off the tile; too small to matter under 32px.
  if (size >= 32) {
    ctx.save();
    ctx.filter = `blur(${1.1 * unit}px)`;
    ctx.translate(0, 0.8 * unit);
    withMark(ctx, size, markBox, () => {
      ctx.fillStyle = "rgba(0,0,0,0.3)";
      for (const { path } of paths) ctx.fill(path);
    });
    ctx.restore();
  }
  // Facets in on-brand tones, glazed from the shade tint at the stem to pure ink at the tip.
  withMark(ctx, size, markBox, () => {
    const glaze = ctx.createLinearGradient(viewBox[0], viewBox[1] + viewBox[3], viewBox[0] + viewBox[2], viewBox[1]);
    glaze.addColorStop(0, paint.inkShade);
    glaze.addColorStop(1, paint.ink);
    ctx.fillStyle = glaze;
    for (const { path, opacity } of paths) {
      ctx.globalAlpha = opacity;
      ctx.fill(path);
    }
    ctx.globalAlpha = 1;
  });
  ctx.restore();
  return canvas;
}

function renderTray(canvasModule, size) {
  const { createCanvas, Path2D } = canvasModule;
  const canvas = createCanvas(size, size);
  const ctx = canvas.getContext("2d");
  withMark(ctx, size, trayMarkBox, () => {
    ctx.fillStyle = paint.tray;
    for (const part of mark.paths) ctx.fill(new Path2D(part.d));
  });
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
  fs.writeFileSync(path.join(assetsRoot, "appicon.png"), png(renderApplication(canvasModule, appSize)));
  fs.writeFileSync(path.join(assetsRoot, "tray.png"), png(renderTray(canvasModule, traySize)));
  fs.writeFileSync(path.join(assetsRoot, "icon.ico"), encodeIco(sizes.map((size) => ({ size, png: png(renderApplication(canvasModule, size)) }))));
  for (const [file, sourceProvenance] of Object.entries(provenance)) {
    const assetPath = path.join(assetsRoot, file);
    fs.writeFileSync(assetPath, writePngProvenance(fs.readFileSync(assetPath), sourceProvenance));
  }
  fs.writeFileSync(manifestPath, JSON.stringify({
    inputs,
    outputs: {
      "appicon.png": { width: appSize, height: appSize, alpha: true, role: "application", treatment: "glazed fold-leaf mark in on-brand tones over a celadon tile with transparent padding" },
      "tray.png": { width: traySize, height: traySize, alpha: true, role: "system tray", treatment: "fold-leaf geometry as an opaque silhouette" },
      "icon.ico": { sizes, role: "Windows executable resource", treatment: "application composition rendered natively at every frame size" },
    },
    sha256: fingerprints(),
  }, null, 2) + "\n");
}

verify();
