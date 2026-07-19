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
      viewBox="0 0 24 24"
    >
      <path d="M9 3H5a2 2 0 0 0-2 2v4M15 3h4a2 2 0 0 1 2 2v4M21 15v4a2 2 0 0 1-2 2h-4M9 21H5a2 2 0 0 1-2-2v-4" />
      <circle cx="12" cy="12" r="2.25" />
    </svg>
  );
}
