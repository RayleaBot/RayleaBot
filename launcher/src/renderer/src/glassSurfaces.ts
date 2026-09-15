/**
 * Liquid Glass surfaces. In WebView2 every glass size gets one SVG backdrop filter that refracts the
 * backdrop through a rounded bevel, tints it and lights the rim. WebKit, Gecko, reduced transparency
 * and forced colors keep the CSS material from liquid-glass.css. Elements opt in with
 * `data-glass="clear" | "regular" | "prominent"`.
 */

export type GlassVariant = "clear" | "regular" | "prominent";

type GlassSpec = {
  width: number;
  height: number;
  radius: number;
  variant: GlassVariant;
  scale: number;
  light: number;
  shade: number;
  glow: number;
  under: number;
  flood: string;
  floodAlpha: number;
};

type Geometry = {
  size: number;
  bevel: number;
  gap: number;
  maxShift: number;
  lineWidth: number;
  glowWidth: number;
  shadeCenter: number;
  shadeWidth: number;
  edgeDarkness: number;
};

type Edge = { depth: number; nx: number; ny: number };

type FilterRecord<T> = {
  id: string;
  resource: T;
  ready: Promise<boolean>;
  users: number;
  releaseTimer: ReturnType<typeof setTimeout> | undefined;
};

type SurfaceState = {
  key: string | null;
  pending: string | null;
  token: number;
  settleTimer: ReturnType<typeof setTimeout> | undefined;
};

const SVG_NS = "http://www.w3.org/2000/svg";
const REFRACTIVE_INDEX = 1.5;
const PROFILE_STEPS = 512;
const RESIZE_SETTLE_MS = 160;
const FILTER_RELEASE_MS = 4000;
const MAX_MAP_PIXELS = 2_500_000;

const materials: Record<GlassVariant, { frost: number; saturate: number }> = {
  clear: { frost: 0, saturate: 1.05 },
  regular: { frost: 3, saturate: 1.25 },
  prominent: { frost: 1.5, saturate: 1.1 },
};

// Light arrives from above and slightly left; rims facing it carry the specular line.
const lightDirection = (() => {
  const length = Math.hypot(-0.28, -1);
  return { x: -0.28 / length, y: -1 / length };
})();

const clamp = (value: number, low: number, high: number) => Math.min(high, Math.max(low, value));

function smoothstep(edge0: number, edge1: number, value: number) {
  const t = clamp((value - edge0) / (edge1 - edge0), 0, 1);
  return t * t * (3 - 2 * t);
}

const squircle = (x: number) => Math.pow(Math.max(0, 1 - Math.pow(1 - x, 4)), 0.25);

function squircleSlope(x: number) {
  const low = Math.max(0, x - 1e-4);
  const high = Math.min(1, x + 1e-4);
  return (squircle(high) - squircle(low)) / (high - low);
}

/**
 * Inward sampling offset for a view ray that meets the bevel at `x = depth / bevel`. The glass floats
 * `gap` pixels above the content, so rays near the edge leave the underside at a grazing angle and land
 * far inside; that content is compressed into the thin colored band just inside the rim.
 */
function rimShift(x: number, geometry: Geometry) {
  if (x >= 1) return 0;
  const incidence = Math.atan(squircleSlope(Math.max(x, 1e-4)));
  const deviation = incidence - Math.asin(Math.sin(incidence) / REFRACTIVE_INDEX);
  const exit = REFRACTIVE_INDEX * Math.sin(deviation);
  if (exit >= 0.9995) return Number.POSITIVE_INFINITY;
  return geometry.bevel * squircle(x) * Math.tan(deviation) + geometry.gap * Math.tan(Math.asin(exit));
}

function geometryFor(width: number, height: number): Geometry {
  const size = Math.min(width, height);
  const bevel = clamp(size * 0.19, 4, 36);
  return {
    size,
    bevel,
    gap: bevel * 1.1,
    maxShift: clamp(size * 0.36, 4, 64),
    lineWidth: clamp(size * 0.011, 1.2, 2),
    glowWidth: clamp(size * 0.035, 1.2, 7),
    shadeCenter: clamp(size * 0.1, 2, 18),
    shadeWidth: clamp(size * 0.09, 2, 16),
    // Small lenses carry a crisp dark side line; on large panels it fades to a hairline.
    edgeDarkness: 0.42 - 0.24 * smoothstep(60, 300, size),
  };
}

/** Distance inside the rounded rectangle and the outward normal of its nearest edge. */
function measureEdge(edge: Edge, px: number, py: number, halfWidth: number, halfHeight: number, radius: number) {
  const qx = Math.abs(px) - (halfWidth - radius);
  const qy = Math.abs(py) - (halfHeight - radius);
  const ox = Math.max(qx, 0);
  const oy = Math.max(qy, 0);
  const outside = Math.hypot(ox, oy);
  edge.depth = radius - outside - Math.min(Math.max(qx, qy), 0);
  edge.nx = 0;
  edge.ny = 0;
  if (outside > 0) {
    edge.nx = (Math.sign(px) * ox) / outside;
    edge.ny = (Math.sign(py) * oy) / outside;
  } else if (qx > qy) {
    edge.nx = Math.sign(px) || 1;
  } else {
    edge.ny = Math.sign(py) || 1;
  }
  return edge;
}

function encodeImage(width: number, height: number, fill: (data: Uint8ClampedArray) => void) {
  const canvas = document.createElement("canvas");
  canvas.width = width;
  canvas.height = height;
  const context = canvas.getContext("2d");
  if (!context) return null;
  const image = context.createImageData(width, height);
  fill(image.data);
  context.putImageData(image, 0, 0);
  return canvas.toDataURL();
}

// R and G push each backdrop sample toward the center; 128 means no shift.
function displacementMap(spec: GlassSpec, geometry: Geometry) {
  const profile = new Float32Array(PROFILE_STEPS + 1);
  for (let i = 0; i <= PROFILE_STEPS; i += 1) {
    profile[i] = Math.min(rimShift(i / PROFILE_STEPS, geometry), geometry.maxShift) / geometry.maxShift;
  }
  const { width, height, radius } = spec;
  const halfWidth = width / 2;
  const halfHeight = height / 2;
  const edge: Edge = { depth: 0, nx: 0, ny: 0 };
  return encodeImage(width, height, (data) => {
    for (let y = 0; y < height; y += 1) {
      for (let x = 0; x < width; x += 1) {
        measureEdge(edge, x + 0.5 - halfWidth, y + 0.5 - halfHeight, halfWidth, halfHeight, radius);
        const step = Math.min(PROFILE_STEPS, Math.round((edge.depth / geometry.bevel) * PROFILE_STEPS));
        const magnitude = edge.depth > 0 && edge.depth < geometry.bevel ? profile[step] ?? 0 : 0;
        const index = (y * width + x) * 4;
        data[index] = Math.round(127.5 - edge.nx * magnitude * 127.5);
        data[index + 1] = Math.round(127.5 - edge.ny * magnitude * 127.5);
        data[index + 2] = 128;
        data[index + 3] = 255;
      }
    }
  });
}

// Drawn at device resolution so the rim line stays crisp. R: specular rim, laid over as white.
// G: bevel shading and the shadow seen through the glass, laid over as black. B: caustic glow, added.
function lightingMap(spec: GlassSpec, geometry: Geometry) {
  const { width, height, radius, scale } = spec;
  const mapWidth = Math.max(1, Math.round(width * scale));
  const mapHeight = Math.max(1, Math.round(height * scale));
  const halfWidth = width / 2;
  const halfHeight = height / 2;
  const soft = 0.5 / scale;
  const underShadow = spec.under * 0.07 * (1 - smoothstep(120, 360, geometry.size));
  const edge: Edge = { depth: 0, nx: 0, ny: 0 };
  return encodeImage(mapWidth, mapHeight, (data) => {
    for (let y = 0; y < mapHeight; y += 1) {
      const py = (y + 0.5) / scale - halfHeight;
      const under = underShadow * smoothstep(-0.9, 0.3, py / halfHeight);
      for (let x = 0; x < mapWidth; x += 1) {
        const index = (y * mapWidth + x) * 4;
        data[index + 3] = 255;
        measureEdge(edge, (x + 0.5) / scale - halfWidth, py, halfWidth, halfHeight, radius);
        if (edge.depth <= 0) continue;
        const facing = edge.nx * lightDirection.x + edge.ny * lightDirection.y;
        const lit = Math.max(facing, 0);
        const back = Math.max(-facing, 0);
        const side = 1 - Math.abs(facing);
        const line = 1 - smoothstep(geometry.lineWidth - soft, geometry.lineWidth + soft, edge.depth);
        const sideLine = 1 - smoothstep(geometry.lineWidth * 0.7 - soft, geometry.lineWidth * 0.7 + soft, edge.depth);
        const rim = line * (lit ** 3 + 0.85 * back ** 3) * 1.2 * spec.light;
        const glow = Math.exp(-edge.depth / geometry.glowWidth) * (lit ** 2 + 0.55 * back ** 2) * (1 - line);
        const bump = Math.exp(-(((edge.depth - geometry.shadeCenter) / geometry.shadeWidth) ** 2));
        const shade = (sideLine * geometry.edgeDarkness * side ** 2 + bump * 0.02 + under * smoothstep(0, geometry.bevel, edge.depth)) * spec.shade;
        data[index] = Math.round(clamp(rim, 0, 1) * 255);
        data[index + 1] = Math.round(clamp(shade, 0, 1) * 255);
        data[index + 2] = Math.round(clamp(glow, 0, 1) * 255);
      }
    }
  });
}

function svgElement(tag: string, attributes: Record<string, string | number>) {
  const node = document.createElementNS(SVG_NS, tag);
  for (const [name, value] of Object.entries(attributes)) node.setAttribute(name, String(value));
  return node;
}

function decodeImage(url: string) {
  const image = new Image();
  image.src = url;
  return image.decode();
}

function createFilter(spec: GlassSpec, id: string, host: Element) {
  const geometry = geometryFor(spec.width, spec.height);
  const filter = svgElement("filter", {
    id,
    x: 0,
    y: 0,
    width: spec.width,
    height: spec.height,
    filterUnits: "userSpaceOnUse",
    primitiveUnits: "userSpaceOnUse",
    "color-interpolation-filters": "sRGB",
  });
  host.append(filter);
  const displacement = displacementMap(spec, geometry);
  const lighting = displacement ? lightingMap(spec, geometry) : null;
  if (!displacement || !lighting) return { resource: filter, ready: Promise.resolve(false) };

  const add = (tag: string, attributes: Record<string, string | number>) => filter.append(svgElement(tag, attributes));
  const material = materials[spec.variant];
  const image = { x: 0, y: 0, width: spec.width, height: spec.height, preserveAspectRatio: "none" };
  let source = "SourceGraphic";
  if (material.frost > 0) {
    add("feGaussianBlur", { in: "SourceGraphic", stdDeviation: material.frost, edgeMode: "duplicate", result: "frost" });
    source = "frost";
  }
  add("feImage", { ...image, href: displacement, result: "map" });
  add("feDisplacementMap", { in: source, in2: "map", scale: (2 * geometry.maxShift).toFixed(2), xChannelSelector: "R", yChannelSelector: "G", result: "lens" });
  // The displacement samples whole pixels, which steps where the rim compresses the backdrop.
  add("feGaussianBlur", { in: "lens", stdDeviation: 0.4, edgeMode: "duplicate", result: "smooth" });
  add("feColorMatrix", { in: "smooth", type: "saturate", values: material.saturate, result: "vivid" });
  let base = "vivid";
  if (spec.flood && spec.floodAlpha > 0) {
    add("feFlood", { "flood-color": spec.flood, "flood-opacity": spec.floodAlpha, result: "tint" });
    add("feComposite", { in: "tint", in2: "vivid", operator: "over", result: "tinted" });
    base = "tinted";
  }
  add("feImage", { ...image, href: lighting, result: "light" });
  add("feColorMatrix", { in: "light", type: "matrix", values: "0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 1 0 0 0", result: "shadeLayer" });
  add("feComposite", { in: "shadeLayer", in2: base, operator: "over", result: "shaded" });
  add("feColorMatrix", { in: "light", type: "matrix", values: "0 0 0 0 1 0 0 0 0 1 0 0 0 0 1 0 0 1 0 0", result: "glowLayer" });
  add("feComposite", { in: "glowLayer", in2: "shaded", operator: "arithmetic", k1: 0, k2: spec.glow, k3: 1, k4: 0, result: "lit" });
  add("feColorMatrix", { in: "light", type: "matrix", values: "0 0 0 0 1 0 0 0 0 1 0 0 0 0 1 1 0 0 0 0", result: "rimLayer" });
  add("feComposite", { in: "rimLayer", in2: "lit", operator: "over" });

  const ready = Promise.all([decodeImage(displacement), decodeImage(lighting)]).then(() => true, () => false);
  return { resource: filter, ready };
}

/** Shares one filter per glass size and removes filters a while after the last surface lets go. */
export class GlassFilterRegistry<T> {
  private readonly records = new Map<string, FilterRecord<T>>();
  private sequence = 0;

  constructor(
    private readonly create: (id: string, key: string) => { resource: T; ready: Promise<boolean> },
    private readonly destroy: (resource: T) => void,
    private readonly releaseDelay = FILTER_RELEASE_MS,
  ) {}

  get size() {
    return this.records.size;
  }

  acquire(key: string): FilterRecord<T> {
    let record = this.records.get(key);
    if (!record) {
      const id = `launcher-glass-${(this.sequence += 1)}`;
      record = { id, users: 0, releaseTimer: undefined, ...this.create(id, key) };
      this.records.set(key, record);
    }
    clearTimeout(record.releaseTimer);
    record.releaseTimer = undefined;
    record.users += 1;
    return record;
  }

  release(key: string) {
    const record = this.records.get(key);
    if (!record) return;
    record.users = Math.max(0, record.users - 1);
    if (record.users > 0 || record.releaseTimer !== undefined) return;
    record.releaseTimer = setTimeout(() => {
      record.releaseTimer = undefined;
      if (record.users > 0 || this.records.get(key) !== record) return;
      this.records.delete(key);
      this.destroy(record.resource);
    }, this.releaseDelay);
  }

  clear() {
    for (const record of this.records.values()) {
      clearTimeout(record.releaseTimer);
      this.destroy(record.resource);
    }
    this.records.clear();
  }
}

/** WebKit and Gecko do not take an SVG filter in backdrop-filter; they keep the CSS material. */
function supportsGlassRefraction(): boolean {
  if (typeof CSS === "undefined" || typeof CSS.supports !== "function") return false;
  if (CSS.supports("-webkit-backdrop-filter", "blur(1px)") || CSS.supports("-moz-appearance", "none")) return false;
  if (!CSS.supports("backdrop-filter", "blur(1px)")) return false;
  return !window.matchMedia?.("(prefers-reduced-transparency: reduce)").matches
    && !window.matchMedia?.("(forced-colors: active)").matches;
}

function parseVariant(value: string | undefined): GlassVariant | null {
  return value === "clear" || value === "regular" || value === "prominent" ? value : null;
}

function numberProperty(style: CSSStyleDeclaration, name: string, fallback: number) {
  const value = Number.parseFloat(style.getPropertyValue(name));
  return Number.isFinite(value) ? value : fallback;
}

function specFor(element: HTMLElement, variant: GlassVariant): GlassSpec | null {
  const width = Math.round(element.offsetWidth);
  const height = Math.round(element.offsetHeight);
  const scale = clamp(Math.round((window.devicePixelRatio || 1) * 4) / 4, 1, 3);
  if (width < 2 || height < 2 || width * height * scale * scale > MAX_MAP_PIXELS) return null;
  const style = getComputedStyle(element);
  const firstRadius = style.borderTopLeftRadius.split(" ")[0] ?? "0";
  const radiusValue = Number.parseFloat(firstRadius) || 0;
  const radius = firstRadius.endsWith("%") ? (radiusValue / 100) * Math.min(width, height) : radiusValue;
  const flood = style.getPropertyValue("--glass-flood").trim();
  return {
    width,
    height,
    radius: Math.round(Math.min(radius, width / 2, height / 2) * 4) / 4,
    variant,
    scale,
    light: numberProperty(style, "--glass-light", 1),
    shade: numberProperty(style, "--glass-shade", 1),
    glow: numberProperty(style, "--glass-glow", 0.14),
    under: numberProperty(style, "--glass-under", 0),
    flood: flood === "none" ? "" : flood,
    floodAlpha: numberProperty(style, "--glass-flood-alpha", 0),
  };
}

/** Tracks `[data-glass]` elements under `root` and keeps each one on the filter for its current size. */
export function startGlassSurfaces(root: HTMLElement): () => void {
  if (typeof ResizeObserver === "undefined" || typeof MutationObserver === "undefined") {
    return () => undefined;
  }

  const host = svgElement("svg", { width: 0, height: 0, "aria-hidden": "true", focusable: "false", class: "glass-filter-host" });
  const defs = svgElement("defs", {});
  host.append(defs);
  document.body.append(host);

  const registry = new GlassFilterRegistry<Element>(
    (id, key) => createFilter(JSON.parse(key) as GlassSpec, id, defs),
    (filter) => filter.remove(),
  );
  const surfaces = new Map<HTMLElement, SurfaceState>();
  let enabled = supportsGlassRefraction();

  const detach = (element: HTMLElement, state: SurfaceState) => {
    state.token += 1;
    state.pending = null;
    clearTimeout(state.settleTimer);
    state.settleTimer = undefined;
    if (state.key) registry.release(state.key);
    state.key = null;
    element.style.removeProperty("--glass-filter");
    delete element.dataset.glassReady;
  };

  const refresh = (element: HTMLElement, settled: boolean) => {
    const state = surfaces.get(element);
    if (!state) return;
    const variant = parseVariant(element.dataset.glass);
    const spec = enabled && variant && element.isConnected ? specFor(element, variant) : null;
    if (!spec) {
      detach(element, state);
      return;
    }
    const key = JSON.stringify(spec);
    if (key === state.key || key === state.pending) return;
    if (!settled && (state.key || state.pending || state.settleTimer !== undefined)) {
      // The size is still changing: show the CSS material until it settles, then build once.
      detach(element, state);
      state.settleTimer = setTimeout(() => {
        state.settleTimer = undefined;
        refresh(element, true);
      }, RESIZE_SETTLE_MS);
      return;
    }
    const token = (state.token += 1);
    state.pending = key;
    const record = registry.acquire(key);
    void record.ready.then((ok) => {
      if (state.token !== token || surfaces.get(element) !== state || !ok) {
        registry.release(key);
        return;
      }
      const previous = state.key;
      state.pending = null;
      state.key = key;
      element.style.setProperty("--glass-filter", `url(#${record.id})`);
      element.dataset.glassReady = "";
      if (previous) registry.release(previous);
    });
  };

  const refreshAll = () => {
    for (const element of surfaces.keys()) refresh(element, true);
  };

  const resizeObserver = new ResizeObserver((entries) => {
    for (const entry of entries) refresh(entry.target as HTMLElement, false);
  });

  const attach = (element: HTMLElement) => {
    if (surfaces.has(element)) return;
    surfaces.set(element, { key: null, pending: null, token: 0, settleTimer: undefined });
    resizeObserver.observe(element);
  };

  const forget = (element: HTMLElement) => {
    const state = surfaces.get(element);
    if (!state) return;
    detach(element, state);
    surfaces.delete(element);
    resizeObserver.unobserve(element);
  };

  const visitGlass = (node: Node, visit: (element: HTMLElement) => void) => {
    if (!(node instanceof HTMLElement)) return;
    if (node.dataset.glass !== undefined) visit(node);
    node.querySelectorAll<HTMLElement>("[data-glass]").forEach(visit);
  };

  const mutationObserver = new MutationObserver((records) => {
    for (const record of records) {
      if (record.type === "attributes") {
        const element = record.target as HTMLElement;
        if (element.dataset.glass === undefined) forget(element);
        else if (surfaces.has(element)) refresh(element, true);
        else attach(element);
        continue;
      }
      record.removedNodes.forEach((node) => visitGlass(node, (element) => {
        if (!element.isConnected) forget(element);
      }));
      record.addedNodes.forEach((node) => visitGlass(node, attach));
    }
  });
  mutationObserver.observe(root, { subtree: true, childList: true, attributes: true, attributeFilter: ["data-glass"] });

  // Theme colors and intensities are read from CSS, so a theme switch rebuilds the filters.
  const themeObserver = new MutationObserver(refreshAll);
  themeObserver.observe(document.documentElement, { attributes: true, attributeFilter: ["data-theme"] });

  const preferenceQueries = ["(prefers-reduced-transparency: reduce)", "(forced-colors: active)"]
    .map((query) => window.matchMedia?.(query))
    .filter((query): query is MediaQueryList => Boolean(query));
  const onPreferenceChange = () => {
    enabled = supportsGlassRefraction();
    refreshAll();
  };
  preferenceQueries.forEach((query) => query.addEventListener("change", onPreferenceChange));

  let scaleQuery: MediaQueryList | undefined;
  const onScaleChange = () => {
    watchScale();
    refreshAll();
  };
  const watchScale = () => {
    scaleQuery?.removeEventListener("change", onScaleChange);
    scaleQuery = window.matchMedia?.(`(resolution: ${window.devicePixelRatio || 1}dppx)`);
    scaleQuery?.addEventListener("change", onScaleChange);
  };
  watchScale();

  root.querySelectorAll<HTMLElement>("[data-glass]").forEach(attach);

  return () => {
    mutationObserver.disconnect();
    themeObserver.disconnect();
    resizeObserver.disconnect();
    preferenceQueries.forEach((query) => query.removeEventListener("change", onPreferenceChange));
    scaleQuery?.removeEventListener("change", onScaleChange);
    for (const [element, state] of surfaces) detach(element, state);
    surfaces.clear();
    registry.clear();
    host.remove();
  };
}
