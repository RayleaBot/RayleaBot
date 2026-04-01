package render

import (
	"context"
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

	actions := []chromedp.Action{
		emulation.SetDeviceMetricsOverride(int64(doc.Width), int64(doc.Height), 1, false),
		chromedp.Navigate(documentURL),
		chromedp.WaitReady("body"),
		chromedp.ActionFunc(func(ctx context.Context) error {
			params := page.CaptureScreenshot()
			if doc.Output == "jpeg" {
				params = params.WithFormat(page.CaptureScreenshotFormatJpeg).WithQuality(90)
			} else {
				params = params.WithFormat(page.CaptureScreenshotFormatPng)
			}
			var err error
			content, err = params.Do(ctx)
			return err
		}),
	}

	if err := chromedp.Run(browserCtx, actions...); err != nil {
		return nil, err
	}
	return content, nil
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
