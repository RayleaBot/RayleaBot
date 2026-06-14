package systemapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"unicode/utf8"
)

func decodeRecoveryConfirmRequest(r *http.Request) (recoveryConfirmRequest, error) {
	if r == nil || r.Body == nil {
		return recoveryConfirmRequest{}, io.EOF
	}
	var req recoveryConfirmRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return recoveryConfirmRequest{}, err
		}
		return recoveryConfirmRequest{}, err
	}
	return req, nil
}

func normalizeRecoveryConfirmRequest(req recoveryConfirmRequest) ([]string, string, bool) {
	reviewIDs := make([]string, 0, len(req.ReviewIDs))
	seen := map[string]struct{}{}
	for _, reviewID := range req.ReviewIDs {
		reviewID = strings.TrimSpace(reviewID)
		if reviewID == "" {
			return nil, "", false
		}
		if _, ok := seen[reviewID]; ok {
			continue
		}
		seen[reviewID] = struct{}{}
		reviewIDs = append(reviewIDs, reviewID)
	}
	if len(reviewIDs) == 0 {
		return nil, "", false
	}
	note := strings.TrimSpace(req.Note)
	if utf8.RuneCountInString(note) > maxRecoveryConfirmNoteRunes {
		return nil, "", false
	}
	return reviewIDs, note, true
}
