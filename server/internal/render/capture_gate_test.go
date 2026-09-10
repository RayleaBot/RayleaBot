package render

import (
	"context"
	"encoding/base64"
	"errors"
	"testing"
	"testing/synctest"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/page"
)

type captureExecutorFunc func(context.Context, string, any, any) error

func (f captureExecutorFunc) Execute(ctx context.Context, method string, params, result any) error {
	return f(ctx, method, params, result)
}

func successfulCaptureExecutor(_ context.Context, method string, _, result any) error {
	if method == page.CommandCaptureScreenshot {
		result.(*page.CaptureScreenshotReturns).Data = base64.StdEncoding.EncodeToString([]byte("screenshot"))
	}
	return nil
}

func TestChromiumCaptureWaitCanBeCancelledWithoutInterruptingOwner(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		runner := NewChromiumRunner(ChromiumOptions{})
		capturing := make(chan struct{})
		release := make(chan struct{})
		firstDone := make(chan error, 1)
		first := cdp.WithExecutor(t.Context(), captureExecutorFunc(func(ctx context.Context, method string, params, result any) error {
			if method == page.CommandCaptureScreenshot {
				close(capturing)
				<-release
			}
			return successfulCaptureExecutor(ctx, method, params, result)
		}))
		go func() { _, err := runner.captureScreenshot(first, "png"); firstDone <- err }()
		<-capturing

		waiting, cancel := context.WithCancel(t.Context())
		waiting = cdp.WithExecutor(waiting, captureExecutorFunc(func(context.Context, string, any, any) error {
			t.Error("another tab reached CDP while the first capture was pending")
			return errors.New("unexpected concurrent capture")
		}))
		secondDone := make(chan error, 1)
		go func() { _, err := runner.captureScreenshot(waiting, "png"); secondDone <- err }()
		synctest.Wait()
		cancel()
		synctest.Wait()
		select {
		case err := <-secondDone:
			if !errors.Is(err, context.Canceled) {
				t.Errorf("waiting capture cancellation: %v", err)
			}
		default:
			t.Error("cancelled request is still waiting for the capture owner")
		}
		select {
		case err := <-firstDone:
			t.Errorf("waiting request interrupted the active capture: %v", err)
		default:
		}
		close(release)
		synctest.Wait()
		if err := <-firstDone; err != nil {
			t.Fatal(err)
		}
		ctx, cancelNext := context.WithTimeout(t.Context(), time.Second)
		defer cancelNext()
		ctx = cdp.WithExecutor(ctx, captureExecutorFunc(successfulCaptureExecutor))
		if _, err := runner.captureScreenshot(ctx, "png"); err != nil {
			t.Fatalf("cancelled waiter lost the next capture slot: %v", err)
		}
	})
}

func TestChromiumCaptureReleasesSlotAfterCDPFailure(t *testing.T) {
	for _, failAt := range []string{page.CommandBringToFront, page.CommandCaptureScreenshot} {
		t.Run(failAt, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				runner := NewChromiumRunner(ChromiumOptions{})
				failure := errors.New("CDP request failed")
				ctx := cdp.WithExecutor(t.Context(), captureExecutorFunc(func(ctx context.Context, method string, params, result any) error {
					if method == failAt {
						return failure
					}
					return successfulCaptureExecutor(ctx, method, params, result)
				}))
				if _, err := runner.captureScreenshot(ctx, "png"); !errors.Is(err, failure) {
					t.Fatalf("capture failure was lost: %v", err)
				}
				next, cancel := context.WithTimeout(t.Context(), time.Second)
				defer cancel()
				next = cdp.WithExecutor(next, captureExecutorFunc(successfulCaptureExecutor))
				if _, err := runner.captureScreenshot(next, "png"); err != nil {
					t.Fatalf("failed capture retained its slot: %v", err)
				}
			})
		})
	}
}
