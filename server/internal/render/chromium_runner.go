package render

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

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

type Document struct {
	Template          string
	Theme             string
	Output            string
	BaseURL           string
	Width             int
	Height            int
	AutoHeight        bool
	DeviceScaleFactor float64
	HTML              string
	Resources         []RenderResource
}

type Runner interface {
	Render(ctx context.Context, doc Document) ([]byte, error)
}

type ChromiumOptions struct {
	BrowserPath    string
	BrowserArgs    []string
	CombinedOutput io.Writer
}

type chromiumRunner struct {
	browserPath    string
	browserArgs    []string
	combinedOutput io.Writer

	mu              sync.Mutex
	closed          bool
	closeErr        error
	profileDir      string
	allocatorCtx    context.Context
	cancelAllocator context.CancelFunc
	command         *exec.Cmd
	browserCtx      context.Context
	cancelBrowser   context.CancelFunc
}

// NewChromiumRunner creates a reusable browser runner. The caller must call Close
// to release the browser process and its temporary profile.
func NewChromiumRunner(options ChromiumOptions) *chromiumRunner {
	return &chromiumRunner{
		browserPath:    strings.TrimSpace(options.BrowserPath),
		browserArgs:    append([]string(nil), options.BrowserArgs...),
		combinedOutput: options.CombinedOutput,
	}
}

func IsChromiumRunner(runner Runner) bool {
	_, ok := runner.(*chromiumRunner)
	return ok
}

func writeTemporaryRenderDocument(html, baseURL string, resources []RenderResource) (string, map[string]string, func(), error) {
	dir, err := os.MkdirTemp("", "rayleabot-render-*")
	if err != nil {
		return "", nil, nil, err
	}

	cleanup := func() {
		_ = os.RemoveAll(dir)
	}

	documentPath := filepath.Join(dir, "document.html")
	if err := os.WriteFile(documentPath, []byte(htmlWithBaseURL(html, baseURL)), 0o600); err != nil {
		cleanup()
		return "", nil, nil, err
	}
	resourceURLs, err := materializeRenderResources(dir, resources)
	if err != nil {
		cleanup()
		return "", nil, nil, err
	}
	return fileURL(documentPath), resourceURLs, cleanup, nil
}

func materializeRenderResources(renderDir string, resources []RenderResource) (map[string]string, error) {
	if len(resources) == 0 {
		return nil, nil
	}
	resourceDir := filepath.Join(renderDir, "resources")
	if err := os.Mkdir(resourceDir, 0o700); err != nil {
		return nil, err
	}
	result := make(map[string]string, len(resources))
	for index, resource := range resources {
		extension, ok := renderResourceExtension(resource.MIME)
		if !ok {
			return nil, fmt.Errorf("render resource %q has unsupported MIME %q", resource.ID, resource.MIME)
		}
		destination := filepath.Join(resourceDir, fmt.Sprintf("%02d%s", index, extension))
		if err := copyVerifiedRenderResource(resource, destination); err != nil {
			return nil, err
		}
		result[resource.ID] = fileURL(destination)
	}
	return result, nil
}

func copyVerifiedRenderResource(resource RenderResource, destination string) error {
	source, err := os.Open(resource.Path)
	if err != nil {
		return fmt.Errorf("open render resource %q: %w", resource.ID, err)
	}
	defer func(release func() error) { _ = release() }(source.Close)

	target, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("create render resource %q: %w", resource.ID, err)
	}
	hash := sha256.New()
	written, copyErr := io.Copy(io.MultiWriter(target, hash), io.LimitReader(source, resource.Size+1))
	closeErr := target.Close()
	if copyErr != nil {
		return fmt.Errorf("copy render resource %q: %w", resource.ID, copyErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close render resource %q: %w", resource.ID, closeErr)
	}
	if written != resource.Size || !strings.EqualFold(hex.EncodeToString(hash.Sum(nil)), resource.SHA256) {
		return fmt.Errorf("render resource %q changed before materialization", resource.ID)
	}
	return nil
}

func bindRenderResourcesExpression(resources map[string]string) (string, error) {
	if len(resources) == 0 {
		return "", nil
	}
	payload, err := json.Marshal(resources)
	if err != nil {
		return "", err
	}
	return `(async () => {
  const resources = ` + string(payload) + `;
  const images = Array.from(document.querySelectorAll("img[data-render-resource]"));
  await Promise.all(images.map(async (image) => {
    const resourceID = image.dataset.renderResource || "";
    const source = resources[resourceID];
    if (!source) {
      return;
    }
    const fallback = image.dataset.fallback || image.getAttribute("src") || "";
    image.src = source;
    try {
      await image.decode();
    } catch (_) {
      if (!fallback) {
        return;
      }
      image.src = new URL(fallback, document.baseURI).href;
      try {
        await image.decode();
      } catch (_) {}
    }
  }));
  return true;
})()`, nil
}

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

func (r *chromiumRunner) Render(ctx context.Context, doc Document) ([]byte, error) {
	browserCtx, err := r.browserContext(ctx)
	if err != nil {
		return nil, err
	}
	tabCtx, cancelTab := chromedp.NewContext(browserCtx)

	runCtx, cancelRun := contextWithRenderDeadline(tabCtx, ctx)
	defer cancelRun()

	if doc.Width <= 0 {
		doc.Width = 960
	}
	if doc.Height <= 0 {
		doc.Height = 640
	}
	deviceScaleFactor := doc.DeviceScaleFactor
	if deviceScaleFactor <= 0 {
		deviceScaleFactor = 1
	}

	renderURL, resourceURLs, cleanup, err := writeTemporaryRenderDocument(doc.HTML, doc.BaseURL, doc.Resources)
	if err != nil {
		cancelTab()
		return nil, err
	}
	defer cleanup()
	defer cancelTab()
	bindResources, err := bindRenderResourcesExpression(resourceURLs)
	if err != nil {
		return nil, err
	}

	var content []byte
	var measuredHeight float64

	actions := []chromedp.Action{
		emulation.SetDeviceMetricsOverride(int64(doc.Width), int64(doc.Height), deviceScaleFactor, false),
		chromedp.Navigate(renderURL),
		chromedp.WaitReady("body"),
	}
	if bindResources != "" {
		actions = append(actions, chromedp.Evaluate(bindResources, nil))
	}
	actions = append(actions, chromedp.Evaluate(waitForLocalAssetsExpression, nil))
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
				return emulation.SetDeviceMetricsOverride(int64(doc.Width), nextHeight, deviceScaleFactor, false).Do(ctx)
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

	if err := chromedp.Run(runCtx, actions...); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		return nil, errors.Join(err, r.resetBrowser())
	}
	return content, nil
}

func (r *chromiumRunner) browserContext(ctx context.Context) (context.Context, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return nil, context.Canceled
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

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
	if r.combinedOutput != nil {
		allocatorOptions = append(allocatorOptions, chromedp.CombinedOutput(r.combinedOutput))
	}
	allocatorOptions = append(allocatorOptions, allocatorFlags(r.browserArgs)...)
	allocatorOptions = append(allocatorOptions, chromedp.ModifyCmdFunc(func(command *exec.Cmd) {
		prepareBrowserCommand(command)
		r.command = command
	}))
	if runtime.GOOS == "windows" && strings.EqualFold(filepath.Base(r.browserPath), "msedge.exe") {
		// Edge's compatibility launcher exits after starting another browser,
		// closing the DevTools output pipe and escaping allocator ownership.
		// Keep the actual browser in the process that the allocator starts.
		allocatorOptions = append(allocatorOptions, chromedp.Flag("edge-skip-compat-layer-relaunch", true))
	}

	// Own only profiles created here. An explicit user-data-dir belongs to the
	// operator and must never be removed by the runner.
	if r.profileDir != "" {
		if err := r.removeProfileLocked(); err != nil {
			return nil, err
		}
	}
	if !hasUserDataDir(r.browserArgs) {
		profileDir, err := os.MkdirTemp("", "rayleabot-chromium-")
		if err != nil {
			return nil, fmt.Errorf("create browser profile: %w", err)
		}
		r.profileDir = profileDir
		allocatorOptions = append(allocatorOptions, chromedp.UserDataDir(profileDir))
	}
	if deadline, ok := ctx.Deadline(); ok {
		// Browser startup shares the caller's render budget. Do not truncate
		// it with chromedp's independent 20-second DevTools URL timeout.
		allocatorOptions = append(allocatorOptions, chromedp.WSURLReadTimeout(time.Until(deadline)))
	}
	allocatorCtx, cancelAllocator := chromedp.NewExecAllocator(context.Background(), allocatorOptions...)
	cancelAllocator = sync.OnceFunc(cancelAllocator)
	browserCtx, cancelBrowser := chromedp.NewContext(allocatorCtx)
	// Startup cancellation and cleanup can race. chromedp cancellation also
	// waits for allocation, so invoke each wait exactly once.
	cancelBrowser = sync.OnceFunc(cancelBrowser)
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
		return nil, errors.Join(err, r.closeLocked())
	}
	close(startDone)

	return r.browserCtx, nil
}

func (r *chromiumRunner) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return r.closeErr
	}
	r.closed = true
	r.closeErr = r.closeLocked()
	return r.closeErr
}

func (r *chromiumRunner) resetBrowser() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.closeLocked()
}

func (r *chromiumRunner) closeLocked() error {
	var closeErr error
	if r.browserCtx != nil && r.browserCtx.Err() == nil && chromedp.FromContext(r.browserCtx).Browser != nil {
		// Let Chrome close its children and profile handles before falling
		// back to the allocator's process cancellation.
		ctx, cancel := context.WithTimeout(r.browserCtx, 2*time.Second)
		closeErr = chromedp.Cancel(ctx)
		cancel()
	}
	if r.cancelBrowser != nil {
		r.cancelBrowser()
	}
	if r.cancelAllocator != nil {
		r.cancelAllocator()
	}
	if r.allocatorCtx != nil {
		// Startup's watcher may already have cancelled before Allocate registered
		// its waiter. Run has returned here; wait again for any registered owner.
		chromedp.FromContext(r.allocatorCtx).Allocator.Wait()
	}
	if r.command != nil && r.command.Process != nil && r.command.ProcessState == nil {
		// Allocate can return on cancellation immediately after Start, before
		// registering cmd.Wait. Reap that process only after the allocator barrier.
		if err := r.command.Wait(); err != nil {
			var exited *exec.ExitError
			if !errors.As(err, &exited) {
				closeErr = errors.Join(closeErr, fmt.Errorf("reap browser process: %w", err))
			}
		}
	}
	r.command = nil
	r.allocatorCtx = nil
	r.cancelAllocator = nil
	r.browserCtx = nil
	r.cancelBrowser = nil
	return errors.Join(closeErr, r.removeProfileLocked())
}

func hasUserDataDir(arguments []string) bool {
	explicit := false
	for _, argument := range arguments {
		key, _, hasValue := strings.Cut(strings.TrimPrefix(strings.TrimSpace(argument), "--"), "=")
		if key == "user-data-dir" {
			explicit = hasValue
		}
	}
	return explicit
}

func (r *chromiumRunner) removeProfileLocked() error {
	if r.profileDir == "" {
		return nil
	}
	// profileDir is assigned exclusively from MkdirTemp above, never from
	// browser arguments. Retry transient Windows handle release within one
	// second, after the allocator has reaped its process.
	deadline := time.Now().Add(time.Second)
	for {
		err := os.RemoveAll(r.profileDir)
		if err == nil {
			r.profileDir = ""
			return nil
		}
		if !time.Now().Before(deadline) {
			return fmt.Errorf("remove browser profile: %w", err)
		}
		time.Sleep(20 * time.Millisecond)
	}
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
