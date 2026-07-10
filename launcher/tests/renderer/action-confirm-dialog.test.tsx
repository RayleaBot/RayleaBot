// @vitest-environment jsdom
import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, test, vi } from "vitest";
import { ActionConfirmDialog } from "@renderer/ActionConfirmDialog";

describe("ActionConfirmDialog", () => {
  test("requires explicit confirmation before installing an update", () => {
    const onConfirm = vi.fn();
    render(<ActionConfirmDialog action="install-update" onCancel={vi.fn()} onConfirm={onConfirm} />);

    expect(screen.getByRole("dialog")).toHaveTextContent("离线备份");
    expect(onConfirm).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole("button", { name: "确认安装" }));
    expect(onConfirm).toHaveBeenCalledWith("install-update");
  });

  test("requires explicit confirmation before resetting administrator credentials", () => {
    const onConfirm = vi.fn();
    render(<ActionConfirmDialog action="reset-admin" onCancel={vi.fn()} onConfirm={onConfirm} />);

    expect(screen.getByRole("dialog")).toHaveTextContent("现有会话");
    expect(onConfirm).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole("button", { name: "确认重置" }));
    expect(onConfirm).toHaveBeenCalledWith("reset-admin");
  });
});
