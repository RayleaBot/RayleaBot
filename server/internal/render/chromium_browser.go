package render

import (
	"context"
	"strings"
	"sync"

	"github.com/chromedp/chromedp"
)

func (r *chromiumRunner) browserContext(ctx context.Context) (context.Context, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.browserCtx != nil {
		return r.browserCtx, nil
	}

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

	allocatorCtx, cancelAllocator := chromedp.NewExecAllocator(context.Background(), allocatorOptions...)
	browserCtx, cancelBrowser := chromedp.NewContext(allocatorCtx)
	r.allocatorCtx = allocatorCtx
	r.cancelAllocator = cancelAllocator
	r.browserCtx = browserCtx
	r.cancelBrowser = cancelBrowser

	startDone := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			cancelBrowser()
			cancelAllocator()
		case <-startDone:
		}
	}()
	if err := chromedp.Run(browserCtx); err != nil {
		close(startDone)
		if ctxErr := ctx.Err(); ctxErr != nil {
			err = ctxErr
		}
		r.closeLocked()
		return nil, err
	}
	close(startDone)

	return r.browserCtx, nil
}

func (r *chromiumRunner) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.closeLocked()
	return nil
}

func (r *chromiumRunner) resetBrowser() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.closeLocked()
}

func (r *chromiumRunner) closeLocked() {
	if r.cancelBrowser != nil {
		r.cancelBrowser()
	}
	if r.cancelAllocator != nil {
		r.cancelAllocator()
	}
	r.allocatorCtx = nil
	r.cancelAllocator = nil
	r.browserCtx = nil
	r.cancelBrowser = nil
}

func contextWithRenderDeadline(parent context.Context, request context.Context) (context.Context, context.CancelFunc) {
	var runCtx context.Context
	var cancel context.CancelFunc
	if deadline, ok := request.Deadline(); ok {
		runCtx, cancel = context.WithDeadline(parent, deadline)
	} else {
		runCtx, cancel = context.WithCancel(parent)
	}

	done := make(chan struct{})
	var once sync.Once
	go func() {
		select {
		case <-request.Done():
			cancel()
		case <-done:
		}
	}()

	return runCtx, func() {
		once.Do(func() {
			close(done)
		})
		cancel()
	}
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
