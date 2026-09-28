type RayleaMarkProps = {
  className?: string;
  tone?: string;
  variant?: "chrome" | "monochrome" | "neutral";
};

export function RayleaMark({ className = "", tone, variant = "monochrome" }: RayleaMarkProps) {
  return (
    <svg
      aria-hidden="true"
      className={`raylea-mark raylea-mark--${variant}${className ? ` ${className}` : ""}`}
      data-tone={tone}
      focusable="false"
      viewBox={rayleaMark.viewBox}
    >
      <path d={rayleaMark.paths[rayleaMark.outline.pathIndex].d} fill="none" stroke={rayleaMark.outline.color} strokeWidth={rayleaMark.outline.width} strokeLinejoin="round" />
      {rayleaMark.paths.map((part) => <path key={part.fill} d={part.d} fill={part.fill} fillRule={part.fillRule} opacity={part.opacity} />)}
    </svg>
  );
}
import { rayleaMark } from "@shared/launcher-theme-tokens.generated";
