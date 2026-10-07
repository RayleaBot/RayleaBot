import { vi } from "vitest";

function normalizedText(value: string | null | undefined) {
  return (value ?? "").replace(/\s+/g, " ").trim();
}

/** The accessible name of a control: the text of the elements labelling it, its aria-label, or its text. */
function controlName(element: Element) {
  const labelIds = element.getAttribute("aria-labelledby")?.split(/\s+/).filter(Boolean) ?? [];
  if (labelIds.length > 0) {
    const root = element.getRootNode() as ParentNode;
    return labelIds.map((id) => normalizedText(root.querySelector(`[id="${id}"]`)?.textContent)).join(" ");
  }
  return element.getAttribute("aria-label") ?? normalizedText(element.textContent);
}

/** Elements whose own text, without their descendants' text, equals the given text. */
export function elementsWithText(text: string, root: ParentNode = document.body): Element[] {
  return [...root.querySelectorAll("*")].filter((element) =>
    normalizedText([...element.childNodes].filter((node) => node.nodeType === Node.TEXT_NODE).map((node) => node.textContent).join("")) === text);
}

export function hasText(text: string, root: ParentNode = document.body) {
  return elementsWithText(text, root).length > 0;
}

export function queryButton(name: string, root: ParentNode = document.body): HTMLButtonElement | null {
  const matches = [...root.querySelectorAll<HTMLButtonElement>("button")].filter((button) => controlName(button) === name);
  if (matches.length > 1) {
    throw new Error(`Found ${matches.length} buttons named ${name}`);
  }
  return matches[0] ?? null;
}

export function getButton(name: string, root: ParentNode = document.body): HTMLButtonElement {
  const button = queryButton(name, root);
  if (!button) {
    throw new Error(`No button named ${name}`);
  }
  return button;
}

export function findButton(name: string, root: ParentNode = document.body) {
  return vi.waitFor(() => getButton(name, root));
}

/** Reka tabs activate on a primary-button press rather than on click. */
export function selectTab(name: string, root: ParentNode = document.body) {
  getButton(name, root).dispatchEvent(new MouseEvent("mousedown", { bubbles: true, button: 0 }));
}

/** The open dialog labelled by the given title, as Reka links them with aria-labelledby, or any open dialog. */
export function queryDialog(name?: string): HTMLElement | null {
  return [...document.querySelectorAll<HTMLElement>("[role=dialog]")].find((dialog) => {
    const label = document.getElementById(dialog.getAttribute("aria-labelledby") ?? "");
    return name === undefined || normalizedText(label?.textContent) === name;
  }) ?? null;
}

export function findDialog(name?: string) {
  return vi.waitFor(() => {
    const dialog = queryDialog(name);
    if (!dialog) throw new Error(`No dialog named ${name}`);
    return dialog;
  });
}

/** The text field labelled by the given name. */
export function getTextbox(name: string, root: ParentNode = document.body): HTMLInputElement {
  const input = [...root.querySelectorAll<HTMLInputElement>("input")].find((candidate) => candidate.getAttribute("aria-label") === name);
  if (!input) {
    throw new Error(`No text field named ${name}`);
  }
  return input;
}
