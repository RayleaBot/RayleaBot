package render

import (
	"context"
	"math"
	"net/url"
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

	documentURL := "data:text/html;charset=utf-8," + url.PathEscape(doc.HTML)
	var content []byte
	var measuredHeight float64

	actions := []chromedp.Action{
		emulation.SetDeviceMetricsOverride(int64(doc.Width), int64(doc.Height), 1, false),
		chromedp.Navigate(documentURL),
		chromedp.WaitReady("body"),
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
