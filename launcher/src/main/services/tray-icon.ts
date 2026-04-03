import { nativeImage } from "electron";

const TRAY_ICON_SVG = `
  <svg xmlns="http://www.w3.org/2000/svg" width="64" height="64">
    <rect width="64" height="64" rx="18" fill="#122032"/>
    <path d="M18 18h28v28H18z" fill="#264763" rx="10"/>
    <path d="M24 22h16c4 0 8 4 8 8v12H36V30c0-2-2-4-4-4h-8z" fill="#7fd6ff"/>
    <circle cx="28" cy="42" r="6" fill="#d6f5ff"/>
  </svg>
`;

export function createTrayImage() {
  return nativeImage.createFromDataURL(`data:image/svg+xml;base64,${Buffer.from(TRAY_ICON_SVG).toString("base64")}`);
}
