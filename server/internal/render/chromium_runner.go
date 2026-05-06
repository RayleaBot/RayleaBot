package render

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

type ChromiumOptions struct {
	BrowserPath string
	BrowserArgs []string
}

type chromiumRunner struct {
	browserPath string
	browserArgs []string
}

func NewChromiumRunner(options ChromiumOptions) Runner {
	return &chromiumRunner{
		browserPath: strings.TrimSpace(options.BrowserPath),
		browserArgs: append([]string(nil), options.BrowserArgs...),
	}
}

func (r *chromiumRunner) Render(ctx context.Context, doc Document) ([]byte, error) {
	allocatorOptions := append([]chromedp.ExecAllocatorOption{}, chromedp.DefaultExecAllocatorOptions[:]...)
	allocatorOptions = append(allocatorOptions,
		chromedp.NoDefaultBrowserCheck,
		chromedp.NoFirstRun,
		chromedp.Headless,
		chromedp.DisableGPU,
	)
	if r.browserPath != "" {
		allocatorOptions = append(allocatorOptions, chromedp.ExecPath(r.browserPath))
	}
	allocatorOptions = append(allocatorOptions, allocatorFlags(r.browserArgs)...)

	allocatorCtx, cancelAllocator := chromedp.NewExecAllocator(ctx, allocatorOptions...)
	defer cancelAllocator()

	browserCtx, cancelBrowser := chromedp.NewContext(allocatorCtx)
	defer cancelBrowser()

	if doc.Width <= 0 {
		doc.Width = 960
	}
	if doc.Height <= 0 {
		doc.Height = 640
	}

	renderURL, cleanup, err := writeTemporaryRenderDocument(doc.HTML, doc.BaseURL)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	var content []byte
	var measuredHeight float64

	actions := []chromedp.Action{
		emulation.SetDeviceMetricsOverride(int64(doc.Width), int64(doc.Height), 1, false),
		chromedp.Navigate(renderURL),
		chromedp.WaitReady("body"),
		chromedp.Evaluate(waitForLocalAssetsExpression, nil),
	}
	if doc.AutoHeight {
		actions = append(actions,
			chromedp.Evaluate(adaptiveDocumentHeightExpression, &measuredHeight),
			chromedp.ActionFunc(func(ctx context.Context) error {
				nextHeight := int64(math.Ceil(measuredHeight))
				if nextHeight < 1 {
					nextHeight = 1
				}
				if nextHeight == int64(doc.Height) {
					return nil
				}
				return emulation.SetDeviceMetricsOverride(int64(doc.Width), nextHeight, 1, false).Do(ctx)
			}),
		)
	}
	actions = append(actions, chromedp.ActionFunc(func(ctx context.Context) error {
		params := page.CaptureScreenshot()
		if doc.Output == "jpeg" {
			params = params.WithFormat(page.CaptureScreenshotFormatJpeg).WithQuality(90)
		} else {
			params = params.WithFormat(page.CaptureScreenshotFormatPng)
		}
		var err error
		content, err = params.Do(ctx)
		return err
	}))

	if err := chromedp.Run(browserCtx, actions...); err != nil {
		return nil, err
	}
	return content, nil
}

func writeTemporaryRenderDocument(html, baseURL string) (string, func(), error) {
	dir, err := os.MkdirTemp("", "rayleabot-render-*")
	if err != nil {
		return "", nil, err
	}

	cleanup := func() {
		_ = os.RemoveAll(dir)
	}

	documentPath := filepath.Join(dir, "document.html")
	if err := os.WriteFile(documentPath, []byte(htmlWithBaseURL(html, baseURL)), 0o600); err != nil {
		cleanup()
		return "", nil, err
	}
	return fileURL(documentPath), cleanup, nil
}

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

var headOpenPattern = regexp.MustCompile(`(?i)<head(\s[^>]*)?>`)

func htmlWithBaseURL(html, baseURL string) string {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" || strings.Contains(strings.ToLower(html), "<base ") {
		return html
	}

	baseElement := `<base href="` + strings.ReplaceAll(baseURL, `"`, "%22") + `">`
	if location := headOpenPattern.FindStringIndex(html); location != nil {
		return html[:location[1]] + baseElement + html[location[1]:]
	}
	return baseElement + html
}

func allocatorFlags(arguments []string) []chromedp.ExecAllocatorOption {
	flags := make([]chromedp.ExecAllocatorOption, 0, len(arguments))
	for _, argument := range arguments {
		argument = strings.TrimSpace(argument)
		argument = strings.TrimPrefix(argument, "--")
		if argument == "" {
			continue
		}
		if key, value, ok := strings.Cut(argument, "="); ok {
			flags = append(flags, chromedp.Flag(key, value))
			continue
		}
		flags = append(flags, chromedp.Flag(argument, true))
	}
	return flags
}
