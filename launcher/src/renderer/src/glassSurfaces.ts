/**
 * Liquid Glass surfaces. Every glass element draws its rim light from a small nine-slice image: a
 * specular line where the edge faces the light, a caustic glow inside it and darker side lines. The
 * image depends only on the corner radius, the element's shorter side and the theme, so surfaces share
 * it and window resizing never redraws it. Clear lenses also refract their backdrop through an SVG
 * backdrop filter in WebView2; the other variants sit on the flat canvas, where refraction would not
 * show, and skip filters entirely. Elements opt in with `data-glass="clear" | "regular" | "prominent"`.
 */

export type GlassVariant = "clear" | "regular" | "prominent";

type LightingSpec = {
  size: number;
  radius: number;
  scale: number;
  compact: boolean;
  light: number;
  shade: number;
  glow: number;
  under: number;
};

type LightingImage = { url: string; slice: number; reach: number };

type LensSpec = { width: number; height: number; radius: number };

type LensGeometry = { bevel: number; gap: number; maxShift: number };

type Edge = { depth: number; nx: number; ny: number };

type FilterRecord<T> = {
  id: string;
  resource: T;
  ready: Promise<boolean>;
  users: number;
  releaseTimer: ReturnType<typeof setTimeout> | undefined;
};

type SurfaceState = {
  lightingKey: string | null;
  lensKey: string | null;
  lensToken: number;
};

const SVG_NS = "http://www.w3.org/2000/svg";
const REFRACTIVE_INDEX = 1.5;
const PROFILE_STEPS = 512;
const FILTER_RELEASE_MS = 4000;
const LIGHTING_CACHE_LIMIT = 48;

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

const lightingCache = new Map<string, LightingImage>();

/**
 * Draws the rim light of a rounded rectangle as a nine-slice image. Straight edges are lit evenly along
 * their length, so the stretched center row and column reproduce any element of the same radius.
 */
function lightingImage(spec: LightingSpec): LightingImage | null {
  const key = JSON.stringify(spec);
  const cached = lightingCache.get(key);
  if (cached) {
    lightingCache.delete(key);
    lightingCache.set(key, cached);
    return cached;
  }

  const lineWidth = clamp(spec.size * 0.011, 1.2, 2);
  const glowWidth = clamp(spec.size * 0.035, 1.2, 7);
  // Small lenses carry a crisp dark side line; on large panels it fades toward a hairline.
  const edgeDarkness = 0.42 - 0.24 * smoothstep(60, 300, spec.size);
  const slice = Math.ceil(Math.max(spec.radius, lineWidth + 1, glowWidth * 5) * spec.scale);
  const pixels = slice * 2 + 1;
  const half = pixels / spec.scale / 2;
  const radius = Math.min(spec.radius, half);
  const soft = 0.5 / spec.scale;
  // Only a lens seen whole carries the shadow cast through it; it would smear across a stretched slice.
  const under = spec.compact ? spec.under * 0.07 * (1 - smoothstep(120, 360, spec.size)) : 0;
  const underDepth = spec.size * 0.19;
  const edge: Edge = { depth: 0, nx: 0, ny: 0 };

  const url = encodeImage(pixels, pixels, (data) => {
    for (let y = 0; y < pixels; y += 1) {
      const py = (y + 0.5) / spec.scale - half;
      const underRow = under * smoothstep(-0.9, 0.3, py / half);
      for (let x = 0; x < pixels; x += 1) {
        measureEdge(edge, (x + 0.5) / spec.scale - half, py, half, half, radius);
        if (edge.depth <= 0) continue;
        const facing = edge.nx * lightDirection.x + edge.ny * lightDirection.y;
        const lit = Math.max(facing, 0);
        const back = Math.max(-facing, 0);
        const side = 1 - Math.abs(facing);
        const line = 1 - smoothstep(lineWidth - soft, lineWidth + soft, edge.depth);
        const sideLine = 1 - smoothstep(lineWidth * 0.7 - soft, lineWidth * 0.7 + soft, edge.depth);
        const rim = clamp(line * (lit ** 3 + 0.85 * back ** 3) * 1.2 * spec.light, 0, 1);
        const glow = clamp(Math.exp(-edge.depth / glowWidth) * (lit ** 2 + 0.55 * back ** 2) * (1 - line) * spec.glow, 0, 1);
        const shade = clamp((sideLine * edgeDarkness * side ** 2 + underRow * smoothstep(0, underDepth, edge.depth)) * spec.shade, 0, 1);
        // Shade, glow and rim stack as black, white and white layers; one gray pixel with alpha holds the result.
        const alpha = 1 - (1 - shade) * (1 - glow) * (1 - rim);
        if (alpha <= 0) continue;
        const gray = Math.round(((rim + glow * (1 - rim)) / alpha) * 255);
        const index = (y * pixels + x) * 4;
        data[index] = gray;
        data[index + 1] = gray;
        data[index + 2] = gray;
        data[index + 3] = Math.round(alpha * 255);
      }
    }
  });
  if (!url) return null;

  const image = { url, slice, reach: slice / spec.scale };
  lightingCache.set(key, image);
  if (lightingCache.size > LIGHTING_CACHE_LIMIT) {
    const oldest = lightingCache.keys().next().value;
    if (oldest !== undefined) lightingCache.delete(oldest);
  }
  return image;
}

/**
 * Inward sampling offset for a view ray that meets the bevel at `x = depth / bevel`. The glass floats
 * `gap` pixels above the content, so rays near the edge leave the underside at a grazing angle and land
 * far inside; that content is compressed into the thin colored band just inside the rim.
 */
function rimShift(x: number, geometry: LensGeometry) {
  if (x >= 1) return 0;
  const incidence = Math.atan(squircleSlope(Math.max(x, 1e-4)));
  const deviation = incidence - Math.asin(Math.sin(incidence) / REFRACTIVE_INDEX);
  const exit = REFRACTIVE_INDEX * Math.sin(deviation);
  if (exit >= 0.9995) return Number.POSITIVE_INFINITY;
  return geometry.bevel * squircle(x) * Math.tan(deviation) + geometry.gap * Math.tan(Math.asin(exit));
}

// R and G push each backdrop sample toward the center; 128 means no shift.
function displacementMap(spec: LensSpec, geometry: LensGeometry) {
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

function svgElement(tag: string, attributes: Record<string, string | number>) {
  const node = document.createElementNS(SVG_NS, tag);
  for (const [name, value] of Object.entries(attributes)) node.setAttribute(name, String(value));
  return node;
}

function createLensFilter(spec: LensSpec, id: string, host: Element) {
  const size = Math.min(spec.width, spec.height);
  const bevel = clamp(size * 0.19, 4, 36);
  const geometry = { bevel, gap: bevel * 1.1, maxShift: clamp(size * 0.36, 4, 64) };
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
  const map = displacementMap(spec, geometry);
  if (!map) return { resource: filter, ready: Promise.resolve(false) };

  const add = (tag: string, attributes: Record<string, string | number>) => filter.append(svgElement(tag, attributes));
  add("feImage", { href: map, x: 0, y: 0, width: spec.width, height: spec.height, preserveAspectRatio: "none", result: "map" });
  add("feDisplacementMap", { in: "SourceGraphic", in2: "map", scale: (2 * geometry.maxShift).toFixed(2), xChannelSelector: "R", yChannelSelector: "G", result: "lens" });
  // The displacement samples whole pixels, which steps where the rim compresses the backdrop.
  add("feGaussianBlur", { in: "lens", stdDeviation: 0.4, edgeMode: "duplicate" });

  const image = new Image();
  image.src = map;
  return { resource: filter, ready: image.decode().then(() => true, () => false) };
}

/** Shares one lens filter per size and removes filters a while after the last surface lets go. */
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

/** WebKit and Gecko do not take an SVG filter in backdrop-filter; their lenses stay clear without refraction. */
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

function radiusOf(style: CSSStyleDeclaration, width: number, height: number) {
  const first = style.borderTopLeftRadius.split(" ")[0] ?? "0";
  const value = Number.parseFloat(first) || 0;
  const radius = first.endsWith("%") ? (value / 100) * Math.min(width, height) : value;
  return Math.round(Math.min(radius, width / 2, height / 2) * 4) / 4;
}

/** Tracks `[data-glass]` elements under `root` and keeps their rim light and lens filter current. */
export function startGlassSurfaces(root: HTMLElement): () => void {
  if (typeof ResizeObserver === "undefined" || typeof MutationObserver === "undefined") {
    return () => undefined;
  }

  let host: Element | null = null;
  const filterHost = () => {
    if (!host) {
      const svg = svgElement("svg", { width: 0, height: 0, "aria-hidden": "true", focusable: "false", class: "glass-filter-host" });
      host = svgElement("defs", {});
      svg.append(host);
      document.body.append(svg);
    }
    return host;
  };
  const registry = new GlassFilterRegistry<Element>(
    (id, key) => createLensFilter(JSON.parse(key) as LensSpec, id, filterHost()),
    (filter) => filter.remove(),
  );
  const surfaces = new Map<HTMLElement, SurfaceState>();
  let refracts = supportsGlassRefraction();

  const clearLighting = (element: HTMLElement, state: SurfaceState) => {
    state.lightingKey = null;
    element.style.removeProperty("--glass-lighting");
    element.style.removeProperty("--glass-slice");
    element.style.removeProperty("--glass-reach");
    delete element.dataset.glassLit;
  };

  const clearLens = (element: HTMLElement, state: SurfaceState) => {
    state.lensToken += 1;
    if (state.lensKey) registry.release(state.lensKey);
    state.lensKey = null;
    element.style.removeProperty("--glass-filter");
    delete element.dataset.glassReady;
  };

  const refresh = (element: HTMLElement) => {
    const state = surfaces.get(element);
    if (!state) return;
    const variant = parseVariant(element.dataset.glass);
    const width = Math.round(element.offsetWidth);
    const height = Math.round(element.offsetHeight);
    if (!variant || !element.isConnected || width < 2 || height < 2) {
      clearLighting(element, state);
      clearLens(element, state);
      return;
    }

    const style = getComputedStyle(element);
    const radius = radiusOf(style, width, height);
    const size = Math.min(width, height);
    const lighting: LightingSpec = {
      size,
      radius,
      scale: clamp(Math.round((window.devicePixelRatio || 1) * 4) / 4, 1, 3),
      compact: size <= radius * 2 + 1,
      light: numberProperty(style, "--glass-light", 1),
      shade: numberProperty(style, "--glass-shade", 1),
      glow: numberProperty(style, "--glass-glow", 1),
      under: numberProperty(style, "--glass-under", 0),
    };
    const lightingKey = JSON.stringify(lighting);
    if (lightingKey !== state.lightingKey) {
      const image = lightingImage(lighting);
      if (image) {
        element.style.setProperty("--glass-lighting", `url("${image.url}")`);
        element.style.setProperty("--glass-slice", String(image.slice));
        element.style.setProperty("--glass-reach", `${image.reach}px`);
        element.dataset.glassLit = "";
        state.lightingKey = lightingKey;
      }
    }

    if (variant !== "clear" || !refracts) {
      clearLens(element, state);
      return;
    }
    const lensKey = JSON.stringify({ width, height, radius } satisfies LensSpec);
    if (lensKey === state.lensKey) return;
    const token = (state.lensToken += 1);
    const record = registry.acquire(lensKey);
    void record.ready.then((ok) => {
      if (state.lensToken !== token || surfaces.get(element) !== state || !ok) {
        registry.release(lensKey);
        return;
      }
      const previous = state.lensKey;
      state.lensKey = lensKey;
      element.style.setProperty("--glass-filter", `url(#${record.id})`);
      element.dataset.glassReady = "";
      if (previous) registry.release(previous);
    });
  };

  const refreshAll = () => {
    for (const element of surfaces.keys()) refresh(element);
  };

  // Rim images are keyed by radius and shorter side, so most resize notifications change nothing.
  const resizeObserver = new ResizeObserver((entries) => {
    for (const entry of entries) refresh(entry.target as HTMLElement);
  });

  const attach = (element: HTMLElement) => {
    if (surfaces.has(element)) return;
    surfaces.set(element, { lightingKey: null, lensKey: null, lensToken: 0 });
    resizeObserver.observe(element);
  };

  const forget = (element: HTMLElement) => {
    const state = surfaces.get(element);
    if (!state) return;
    clearLighting(element, state);
    clearLens(element, state);
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
        else if (surfaces.has(element)) refresh(element);
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

  // Theme colors and intensities are read from CSS, so a theme switch redraws the rim images.
  const themeObserver = new MutationObserver(refreshAll);
  themeObserver.observe(document.documentElement, { attributes: true, attributeFilter: ["data-theme"] });

  const preferenceQueries = ["(prefers-reduced-transparency: reduce)", "(forced-colors: active)"]
    .map((query) => window.matchMedia?.(query))
    .filter((query): query is MediaQueryList => Boolean(query));
  const onPreferenceChange = () => {
    refracts = supportsGlassRefraction();
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
    for (const [element, state] of surfaces) {
      clearLighting(element, state);
      clearLens(element, state);
    }
    surfaces.clear();
    registry.clear();
    host?.parentElement?.remove();
  };
}
