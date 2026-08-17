// @vitest-environment jsdom
import { afterEach, describe, expect, test } from "vitest";

import { installTrustedNavigationGuards } from "../../src/renderer/src/trustedNavigation";

afterEach(() => {
  document.body.replaceChildren();
});

describe("trusted renderer navigation", () => {
  test("blocks links, forms, new windows, and all dropped navigation payloads", () => {
    const uninstall = installTrustedNavigationGuards();
    const link = document.createElement("a");
    link.href = "http://example.test/";
    document.body.append(link);
    const click = new MouseEvent("click", { bubbles: true, cancelable: true });
    link.dispatchEvent(click);

    const form = document.createElement("form");
    document.body.append(form);
    const submit = new Event("submit", { bubbles: true, cancelable: true });
    form.dispatchEvent(submit);

    const transfer = { types: ["text/uri-list"], dropEffect: "move" } as unknown as DataTransfer;
    const drop = new Event("drop", { bubbles: true, cancelable: true }) as DragEvent;
    Object.defineProperty(drop, "dataTransfer", { value: transfer });
    document.body.dispatchEvent(drop);

    const fileTransfer = { types: ["Files"], dropEffect: "copy" } as unknown as DataTransfer;
    const fileDragOver = new Event("dragover", { bubbles: true, cancelable: true }) as DragEvent;
    Object.defineProperty(fileDragOver, "dataTransfer", { value: fileTransfer });
    document.body.dispatchEvent(fileDragOver);
    const fileDrop = new Event("drop", { bubbles: true, cancelable: true }) as DragEvent;
    Object.defineProperty(fileDrop, "dataTransfer", { value: fileTransfer });
    document.body.dispatchEvent(fileDrop);

    expect(click.defaultPrevented).toBe(true);
    expect(submit.defaultPrevented).toBe(true);
    expect(drop.defaultPrevented).toBe(true);
    expect(transfer.dropEffect).toBe("none");
    expect(fileDragOver.defaultPrevented).toBe(true);
    expect(fileDrop.defaultPrevented).toBe(true);
    expect(fileTransfer.dropEffect).toBe("none");
    expect(window.open("http://example.test/")).toBeNull();

    uninstall();
  });

  test("leaves ordinary controls available", () => {
    const uninstall = installTrustedNavigationGuards();
    const button = document.createElement("button");
    document.body.append(button);
    const click = new MouseEvent("click", { bubbles: true, cancelable: true });
    button.dispatchEvent(click);

    expect(click.defaultPrevented).toBe(false);

    uninstall();
  });
});
