package render

const adaptiveDocumentHeightExpression = `(() => {
  const body = document.body;
  if (!body) {
    return 1;
  }
  const elements = Array.from(body.querySelectorAll("*"));
  if (body.children.length === 0 && body.textContent.trim()) {
    return Math.max(1, Math.ceil(body.scrollHeight));
  }

  let top = 0;
  let bottom = 0;
  for (const element of elements) {
    const rect = element.getBoundingClientRect();
    if (rect.width === 0 && rect.height === 0) {
      continue;
    }
    top = Math.min(top, rect.top);
    bottom = Math.max(bottom, rect.bottom);
  }

  return Math.max(1, Math.ceil(bottom - Math.min(0, top)));
})()`

const waitForLocalAssetsExpression = `(() => {
  const urls = new Set();
  const addURL = (value) => {
    if (!value || value === "none") {
      return;
    }
    for (const match of value.matchAll(/url\((?:"([^"]+)"|'([^']+)'|([^)]+))\)/g)) {
      const raw = (match[1] || match[2] || match[3] || "").trim();
      if (!raw) {
        continue;
      }
      const absolute = new URL(raw, document.baseURI).href;
      if (absolute.startsWith("file:") || absolute.startsWith("data:")) {
        urls.add(absolute);
      }
    }
  };

  for (const element of document.querySelectorAll("*")) {
    const style = getComputedStyle(element);
    addURL(style.backgroundImage);
    addURL(style.borderImageSource);
    addURL(style.listStyleImage);
    addURL(style.maskImage);
    addURL(style.webkitMaskImage);
    const before = getComputedStyle(element, "::before");
    addURL(before.backgroundImage);
    addURL(before.borderImageSource);
    addURL(before.listStyleImage);
    addURL(before.maskImage);
    addURL(before.webkitMaskImage);
    const after = getComputedStyle(element, "::after");
    addURL(after.backgroundImage);
    addURL(after.borderImageSource);
    addURL(after.listStyleImage);
    addURL(after.maskImage);
    addURL(after.webkitMaskImage);
  }

  for (const image of document.images) {
    if ((image.currentSrc || image.src || "").startsWith("file:") || (image.currentSrc || image.src || "").startsWith("data:")) {
      urls.add(image.currentSrc || image.src);
    }
  }

  const imagesReady = Promise.all(Array.from(urls, (url) => new Promise((resolve) => {
    const image = new Image();
    image.onload = resolve;
    image.onerror = resolve;
    image.src = url;
    if (image.complete) {
      resolve();
    }
  })));

  const fontsReady = document.fonts && document.fonts.ready
    ? document.fonts.ready.catch(() => true)
    : Promise.resolve(true);

  return Promise.all([imagesReady, fontsReady]).then(() => true);
})()`
