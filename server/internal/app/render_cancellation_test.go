package app

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/actions"
)

type canceledActionRenderer struct {
	resolve bool
	failure error
}

func (r canceledActionRenderer) ResolvePluginTemplate(context.Context, string, string) (string, error) {
	if r.resolve {
		return "", r.failure
	}
	return "fixture", nil
}
func (r canceledActionRenderer) RenderImage(context.Context, actions.RenderImageRequest) (actions.RenderImageResult, error) {
	return actions.RenderImageResult{}, r.failure
}
func (canceledActionRenderer) TemplateAcceptsRenderIdentity(context.Context, string) bool {
	return false
}

func TestRenderActionCancellationKeepsCauseWithoutFailureLog(t *testing.T) {
	for _, resolve := range []bool{false, true} {
		for _, canceled := range []bool{false, true} {
			var logs bytes.Buffer
			cause := errors.New("fixture render failure")
			if canceled {
				cause = context.Canceled
			}
			failure := &actions.RenderTemplateError{Code: "platform.internal_error", Message: "fixture", Err: cause}
			s := actions.New(actions.Deps{Logger: slog.New(slog.NewJSONHandler(&logs, nil)), Renderer: canceledActionRenderer{resolve: resolve, failure: failure}})
			_, err := s.Execute(t.Context(), "fixture", "fixture", plugins.Action{Kind: "render.image", RenderTemplate: "fixture"}, chatevent.Event{})
			if !errors.Is(err, cause) {
				t.Fatalf("cause lost: %v", err)
			}
			warned := strings.Contains(logs.String(), `"level":"WARN"`)
			if warned == canceled {
				t.Fatalf("resolve=%v canceled=%v logs=%s", resolve, canceled, logs.String())
			}
		}
	}
}
