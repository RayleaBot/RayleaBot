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
      {rayleaMark.paths.map((part) => <path key={part.d} d={part.d} opacity={part.opacity} />)}
    </svg>
  );
}
import { rayleaMark } from "@shared/launcher-theme-tokens.generated";
