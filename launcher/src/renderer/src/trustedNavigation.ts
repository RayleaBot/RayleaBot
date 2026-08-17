const navigationSelector = "a[href], area[href]";

function isNavigationTarget(target: EventTarget | null): boolean {
  return target instanceof Element && target.closest(navigationSelector) !== null;
}

export function installTrustedNavigationGuards(targetWindow: Window = window): () => void {
  const document = targetWindow.document;
  const blockLinkNavigation = (event: Event) => {
    if (isNavigationTarget(event.target)) {
      event.preventDefault();
    }
  };
  const blockFormNavigation = (event: Event) => event.preventDefault();
  const blockDroppedNavigation = (event: DragEvent) => {
    event.preventDefault();
    if (event.dataTransfer) {
      event.dataTransfer.dropEffect = "none";
    }
  };
  const originalOpen = targetWindow.open;
  const blockedOpen = (() => null) as typeof targetWindow.open;

  document.addEventListener("click", blockLinkNavigation, true);
  document.addEventListener("auxclick", blockLinkNavigation, true);
  document.addEventListener("submit", blockFormNavigation, true);
  document.addEventListener("dragover", blockDroppedNavigation, true);
  document.addEventListener("drop", blockDroppedNavigation, true);
  targetWindow.open = blockedOpen;

  return () => {
    document.removeEventListener("click", blockLinkNavigation, true);
    document.removeEventListener("auxclick", blockLinkNavigation, true);
    document.removeEventListener("submit", blockFormNavigation, true);
    document.removeEventListener("dragover", blockDroppedNavigation, true);
    document.removeEventListener("drop", blockDroppedNavigation, true);
    if (targetWindow.open === blockedOpen) {
      targetWindow.open = originalOpen;
    }
  };
}
