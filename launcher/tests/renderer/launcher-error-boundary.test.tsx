// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { describe, expect, test, vi } from "vitest";
import { LauncherErrorBoundary } from "@renderer/LauncherErrorBoundary";

function BrokenPanel() {
  throw new Error("render failed");
}

describe("LauncherErrorBoundary", () => {
  test("renders a launcher-level fallback when the renderer tree crashes", () => {
    const errorSpy = vi.spyOn(console, "error").mockImplementation(() => undefined);
    const swallowExpectedError = (event: Event) => {
      event.preventDefault();
    };
    window.addEventListener("error", swallowExpectedError);

    try {
      render(
        <LauncherErrorBoundary>
          <BrokenPanel />
        </LauncherErrorBoundary>,
      );
    } finally {
      window.removeEventListener("error", swallowExpectedError);
      errorSpy.mockRestore();
    }

    expect(screen.getByText("启动器界面暂时不可用")).toBeInTheDocument();
    expect(screen.getByText("请关闭后重新打开 Raylea 启动器。")).toBeInTheDocument();
  });
});
