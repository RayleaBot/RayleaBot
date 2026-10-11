package actions

import (
	"context"
	"errors"
	"testing"
	"testing/synctest"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

type budgetTestRenderer struct {
	resolve, render time.Duration
	deadlines       []time.Time
}

func (r *budgetTestRenderer) ResolvePluginTemplate(ctx context.Context, _, _ string) (string, error) {
	d, _ := ctx.Deadline()
	r.deadlines = append(r.deadlines, d)
	select {
	case <-time.After(r.resolve):
		return "fixture", nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}
func (*budgetTestRenderer) TemplateAcceptsRenderIdentity(context.Context, string) bool { return false }
func (r *budgetTestRenderer) RenderImage(ctx context.Context, _ RenderImageRequest) (RenderImageResult, error) {
	d, _ := ctx.Deadline()
	r.deadlines = append(r.deadlines, d)
	select {
	case <-time.After(r.render):
		return RenderImageResult{ImagePath: "fixture.png"}, nil
	case <-ctx.Done():
		return RenderImageResult{}, ctx.Err()
	}
}

func TestRenderActionBudgetIncludesPreparationAndRendering(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		renderer := &budgetTestRenderer{resolve: 20 * time.Millisecond, render: 40 * time.Millisecond}
		started := time.Now()
		result, err := executeRenderImage(t.Context(), Deps{Renderer: renderer}, ActionRequest{Action: plugins.Action{RenderTemplate: "fixture", RenderTimeoutMS: 50}})
		var failure *plugins.Error
		if result != nil || !errors.As(err, &failure) || failure.Code != "platform.render_timeout" {
			t.Fatalf("result=%v err=%v", result, err)
		}
		if time.Since(started) != 50*time.Millisecond || len(renderer.deadlines) != 2 || !renderer.deadlines[0].Equal(renderer.deadlines[1]) {
			t.Fatalf("elapsed=%v deadlines=%v", time.Since(started), renderer.deadlines)
		}
	})
}

func TestRenderActionBudgetCannotExtendParentDeadline(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithTimeout(t.Context(), 10*time.Millisecond)
		defer cancel()
		renderer := &budgetTestRenderer{resolve: time.Second}
		started := time.Now()
		_, err := executeRenderImage(ctx, Deps{Renderer: renderer}, ActionRequest{Action: plugins.Action{RenderTemplate: "fixture", RenderTimeoutMS: 500}})
		if !errors.Is(err, context.DeadlineExceeded) || time.Since(started) != 10*time.Millisecond {
			t.Fatalf("elapsed=%v err=%v", time.Since(started), err)
		}
	})
}
