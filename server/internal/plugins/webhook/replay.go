package webhook

import (
	"net/http"
	"strconv"
	"strings"
	"time"
)

// replayDecision summarises the replay-protection outcome for a single
// webhook request. When reject is false the request continues into HMAC
// validation; the parsed timestamp / event id are reused to assemble the
// downstream plugin event so the plugin sees consistent identifiers. The
// dedup key + ttl are populated when peek-then-commit is in play so the
// caller can mark the (route, event_id) as seen only after authentication
// succeeds.
type replayDecision struct {
	reject       bool
	code         string
	messageKey   string
	timestamp    int64
	timestampRaw string
	eventID      string
	dedupKey     string
	dedupTTL     time.Duration
}

func (s *Service) evaluateReplayProtection(pluginID, route string, cfg ReplayProtection, r *http.Request) replayDecision {
	timestampRaw := strings.TrimSpace(r.Header.Get(cfg.TimestampHeader))
	eventID := strings.TrimSpace(r.Header.Get(cfg.EventIDHeader))
	decision := replayDecision{timestampRaw: timestampRaw, eventID: eventID}

	if timestampRaw == "" || eventID == "" {
		if cfg.Enforce {
			decision.reject = true
			decision.code = "plugin.webhook_replay_rejected"
			decision.messageKey = "errors.plugin.webhook_replay_rejected"
			s.recordReplayMetric("rejected")
		} else {
			s.recordReplayMetric("grace_observed")
		}
		return decision
	}

	timestamp, parseErr := strconv.ParseInt(timestampRaw, 10, 64)
	if parseErr != nil {
		if cfg.Enforce {
			decision.reject = true
			decision.code = "plugin.webhook_timestamp_skew"
			decision.messageKey = "errors.plugin.webhook_timestamp_skew"
			s.recordReplayMetric("skew")
		} else {
			s.recordReplayMetric("grace_observed")
		}
		return decision
	}
	decision.timestamp = timestamp

	now := s.now().Unix()
	tolerance := int64(cfg.ToleranceSeconds)
	if tolerance <= 0 {
		tolerance = 300
	}
	if now-timestamp > tolerance || timestamp-now > tolerance {
		if cfg.Enforce {
			decision.reject = true
			decision.code = "plugin.webhook_timestamp_skew"
			decision.messageKey = "errors.plugin.webhook_timestamp_skew"
			s.recordReplayMetric("skew")
		} else {
			s.recordReplayMetric("grace_observed")
		}
		return decision
	}

	dedupKey := webhookKey(pluginID, route) + "\x00" + eventID
	ttl := time.Duration(2*tolerance) * time.Second
	decision.dedupKey = dedupKey
	decision.dedupTTL = ttl
	if s.dedup.peek(dedupKey, s.now(), ttl) {
		if cfg.Enforce {
			decision.reject = true
			decision.code = "plugin.webhook_replay_rejected"
			decision.messageKey = "errors.plugin.webhook_replay_rejected"
			s.recordReplayMetric("rejected")
		} else {
			s.recordReplayMetric("grace_observed")
		}
		return decision
	}

	return decision
}

func (s *Service) recordReplayMetric(outcome string) {
	if s == nil || s.metrics == nil {
		return
	}
	s.metrics.IncReplayObserved(outcome)
}
