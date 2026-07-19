import { nativeImage } from "electron";
import { launcherThemes } from "../../shared/launcher-theme";

function renderTrayIconSvg() {
  const monochrome = launcherThemes.light.brandFill;
  return `
    <svg xmlns="http://www.w3.org/2000/svg" width="64" height="64" viewBox="0 0 64 64">
      <path d="M25 10H16a6 6 0 0 0-6 6v9M39 10h9a6 6 0 0 1 6 6v9M54 39v9a6 6 0 0 1-6 6h-9M25 54h-9a6 6 0 0 1-6-6v-9" fill="none" stroke="${monochrome}" stroke-linecap="round" stroke-linejoin="round" stroke-width="6"/>
      <circle cx="32" cy="32" r="7" fill="${monochrome}"/>
    </svg>
  `;
}

export function createTrayImage() {
  const svg = renderTrayIconSvg();
  return nativeImage.createFromDataURL(`data:image/svg+xml;base64,${Buffer.from(svg).toString("base64")}`);
}
