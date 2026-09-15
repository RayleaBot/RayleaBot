import {
  useCallback,
  useEffect,
  useId,
  useRef,
  useState,
  type CSSProperties,
  type PointerEvent,
} from "react";

import { prefersReducedMotion } from "./launcherMotion";

type GlassDisplacement = {
  width: number;
  height: number;
  url: string;
};

const lensBevel = 18;
const maxMapEdge = 720;

/** A rounded lens normal map: only the bevel bends light, the center stays clear. */
function createGlassDisplacement(width: number, height: number, radius: number) {
  const ratio = Math.min(1, maxMapEdge / Math.max(width, height));
  const mapWidth = Math.max(1, Math.round(width * ratio));
  const mapHeight = Math.max(1, Math.round(height * ratio));
  const pixels = new Uint8ClampedArray(mapWidth * mapHeight * 4);

  for (let y = 0; y < mapHeight; y += 1) {
    for (let x = 0; x < mapWidth; x += 1) {
      const px = ((x + 0.5) / mapWidth) * width - width / 2;
      const py = ((y + 0.5) / mapHeight) * height - height / 2;
      const qx = Math.abs(px) - (width / 2 - radius);
      const qy = Math.abs(py) - (height / 2 - radius);
      const ox = Math.max(qx, 0);
      const oy = Math.max(qy, 0);
      const length = Math.hypot(ox, oy);
      const distance = radius - length - Math.min(Math.max(qx, qy), 0);
      const bend = distance > 0 && distance < lensBevel ? Math.sin((distance / lensBevel) * Math.PI) ** 0.8 : 0;
      const nx = length > 0 ? (Math.sign(px) * ox) / length : 0;
      const ny = length > 0 ? (Math.sign(py) * oy) / length : 0;
      const index = (y * mapWidth + x) * 4;
      pixels[index] = Math.round(127.5 - nx * bend * 127);
      pixels[index + 1] = Math.round(127.5 - ny * bend * 127);
      pixels[index + 2] = 128;
      pixels[index + 3] = 255;
    }
  }
  return { width: mapWidth, height: mapHeight, pixels };
}

/**
 * WebView2 accepts an SVG filter reference in backdrop-filter. WebKit and Gecko keep the
 * frosted CSS material, and reduced transparency or forced colors drop the backdrop, so
 * the lens raster is only built where it is consumed.
 */
function supportsGlassRefraction(): boolean {
  if (typeof window === "undefined" || typeof CSS === "undefined" || typeof CSS.supports !== "function") {
    return false;
  }
  if (CSS.supports("-webkit-backdrop-filter", "blur(1px)") || CSS.supports("-moz-appearance", "none")) {
    return false;
  }
  if (!CSS.supports("backdrop-filter", "blur(1px)")) {
    return false;
  }
  return !window.matchMedia?.("(prefers-reduced-transparency: reduce)").matches
    && !window.matchMedia?.("(forced-colors: active)").matches;
}

export function useLiquidGlass<T extends HTMLElement>() {
  const surfaceRef = useRef<T | null>(null);
  const filterId = `launcher-glass-${useId().replace(/[^\w-]/g, "")}`;
  const [displacement, setDisplacement] = useState<GlassDisplacement | null>(null);
  const highlight = useRef({ frame: 0, x: 0, y: 0 });

  useEffect(() => {
    const element = surfaceRef.current;
    if (!element || typeof ResizeObserver === "undefined" || !supportsGlassRefraction()) {
      return undefined;
    }

    let frame = 0;
    let renderedSize = "";
    let pendingSize = "";
    let pendingImage: HTMLImageElement | null = null;
    let disposed = false;

    const releasePendingImage = () => {
      if (!pendingImage) return;
      pendingImage.onload = null;
      pendingImage.onerror = null;
      pendingImage.src = "";
      pendingImage = null;
    };

    const refresh = () => {
      frame = 0;
      const width = element.offsetWidth;
      const height = element.offsetHeight;
      if (disposed || !width || !height) return;
      // Compare the box first so an unchanged panel never forces a style recalculation.
      const size = `${width}:${height}`;
      if (size === renderedSize || size === pendingSize) return;
      const radius = Number.parseFloat(getComputedStyle(element).borderTopLeftRadius) || 0;
      const map = createGlassDisplacement(width, height, radius);
      const canvas = document.createElement("canvas");
      canvas.width = map.width;
      canvas.height = map.height;
      const context = canvas.getContext("2d");
      if (!context) return;
      const image = context.createImageData(map.width, map.height);
      image.data.set(map.pixels);
      context.putImageData(image, 0, 0);
      const url = canvas.toDataURL();

      // Keep the CSS material until the map decodes, then swap in the SVG lens.
      releasePendingImage();
      pendingSize = size;
      const decoded = new Image();
      pendingImage = decoded;
      decoded.onload = () => {
        pendingImage = null;
        pendingSize = "";
        if (disposed) return;
        renderedSize = size;
        setDisplacement({ width, height, url });
      };
      decoded.onerror = () => {
        pendingImage = null;
        pendingSize = "";
      };
      decoded.src = url;
    };

    const scheduleRefresh = () => {
      if (!frame) frame = requestAnimationFrame(refresh);
    };

    const observer = new ResizeObserver(scheduleRefresh);
    observer.observe(element);
    scheduleRefresh();

    return () => {
      disposed = true;
      observer.disconnect();
      cancelAnimationFrame(frame);
      releasePendingImage();
    };
  }, []);

  useEffect(() => {
    const state = highlight.current;
    return () => cancelAnimationFrame(state.frame);
  }, []);

  const onPointerMove = useCallback((event: PointerEvent<T>) => {
    if (event.pointerType !== "mouse" || prefersReducedMotion()) return;
    const state = highlight.current;
    state.x = event.clientX;
    state.y = event.clientY;
    if (state.frame) return;
    state.frame = requestAnimationFrame(() => {
      state.frame = 0;
      const element = surfaceRef.current;
      if (!element) return;
      const rect = element.getBoundingClientRect();
      if (!rect.width || !rect.height) return;
      element.style.setProperty("--glass-light-x", `${((state.x - rect.left) / rect.width) * 100}%`);
      element.style.setProperty("--glass-light-y", `${((state.y - rect.top) / rect.height) * 100}%`);
    });
  }, []);

  const onPointerLeave = useCallback(() => {
    const state = highlight.current;
    cancelAnimationFrame(state.frame);
    state.frame = 0;
    surfaceRef.current?.style.removeProperty("--glass-light-x");
    surfaceRef.current?.style.removeProperty("--glass-light-y");
  }, []);

  const refractionStyle = displacement
    ? ({ "--glass-refraction": `url(#${filterId})` } as CSSProperties)
    : undefined;

  return { surfaceRef, filterId, displacement, refractionStyle, onPointerMove, onPointerLeave };
}

type LiquidGlassFilterProps = {
  id: string;
  displacement: GlassDisplacement | null;
};

export function LiquidGlassFilter({ id, displacement }: LiquidGlassFilterProps) {
  return (
    <svg className="liquid-glass-filters" aria-hidden="true" focusable="false">
      <defs>
        {displacement ? (
          <filter
            id={id}
            x="0"
            y="0"
            width={displacement.width}
            height={displacement.height}
            filterUnits="userSpaceOnUse"
            colorInterpolationFilters="sRGB"
          >
            <feGaussianBlur in="SourceGraphic" stdDeviation="1.2" result="softened" />
            <feImage href={displacement.url} x="0" y="0" width={displacement.width} height={displacement.height} result="lens" />
            <feDisplacementMap in="softened" in2="lens" scale="36" xChannelSelector="R" yChannelSelector="G" />
          </filter>
        ) : null}
      </defs>
    </svg>
  );
}
