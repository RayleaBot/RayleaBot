package systemapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

type runtimeBootstrapRequest struct {
	Resources []string `json:"resources,omitempty"`
}

func decodeRuntimeBootstrapRequest(r *http.Request) (runtimeBootstrapRequest, error) {
	if r == nil || r.Body == nil {
		return runtimeBootstrapRequest{}, nil
	}
	var req runtimeBootstrapRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return runtimeBootstrapRequest{}, err
		}
		if err == io.EOF {
			return runtimeBootstrapRequest{}, nil
		}
		return runtimeBootstrapRequest{}, err
	}
	return req, nil
}

func normalizeRuntimeBootstrapResources(requested []string) ([]string, bool) {
	if len(requested) == 0 {
		return []string{"chromium", "python-runtime", "nodejs-runtime"}, true
	}
	seen := map[string]struct{}{}
	resources := make([]string, 0, len(requested))
	for _, item := range requested {
		switch item {
		case "chromium", "python-runtime", "nodejs-runtime":
		default:
			return nil, false
		}
		if _, ok := seen[item]; ok {
			return nil, false
		}
		seen[item] = struct{}{}
		resources = append(resources, item)
	}
	return resources, true
}
