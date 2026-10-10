export const compactRendering = {
  samples: 4,
  fit: "viewBox",
};

export function renderLauncherMark(canvasModule, mark, size, mode = "light", compact = false) {
  const { createCanvas, Path2D } = canvasModule;
  const viewBox = mark.viewBox.trim().split(/[\s,]+/).map(Number);
  const theme = mark.themes[mode];
  const samples = compact ? compactRendering.samples : 1;
  const paths = theme.paths.map((part) => new Path2D(part.d));
  const sourceSize = size * samples;
  const canvas = createCanvas(sourceSize, sourceSize);
  const ctx = canvas.getContext("2d");
  const scale = sourceSize / Math.max(viewBox[2], viewBox[3]);
  ctx.translate((sourceSize - viewBox[2] * scale) / 2 - viewBox[0] * scale, (sourceSize - viewBox[3] * scale) / 2 - viewBox[1] * scale);
  ctx.scale(scale, scale);
  if (theme.outline) {
    ctx.strokeStyle = theme.outline.color;
    ctx.lineWidth = theme.outline.width;
    ctx.lineJoin = "round";
    ctx.stroke(paths[theme.outline.pathIndex]);
  }
  for (const [index, part] of theme.paths.entries()) {
    ctx.fillStyle = part.fill;
    ctx.globalAlpha = part.opacity;
    ctx.fill(paths[index], part.fillRule);
  }
  if (!compact) return canvas;

  const input = ctx.getImageData(0, 0, sourceSize, sourceSize).data;
  const result = createCanvas(size, size);
  const target = result.getContext("2d");
  const pixels = target.createImageData(size, size);
  for (let y = 0; y < size; y++) for (let x = 0; x < size; x++) {
    let alpha = 0;
    const color = [0, 0, 0];
    for (let dy = 0; dy < samples; dy++) for (let dx = 0; dx < samples; dx++) {
      const index = ((y * samples + dy) * sourceSize + x * samples + dx) * 4;
      const a = input[index + 3];
      alpha += a;
      for (let channel = 0; channel < 3; channel++) color[channel] += input[index + channel] * a;
    }
    const index = (y * size + x) * 4;
    for (let channel = 0; channel < 3; channel++) pixels.data[index + channel] = alpha ? Math.round(color[channel] / alpha) : 0;
    pixels.data[index + 3] = Math.round(alpha / (samples * samples));
  }
  target.putImageData(pixels, 0, 0);
  return result;
}
