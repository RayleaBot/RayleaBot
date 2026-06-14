package logging

import (
	"bytes"
	"encoding/json"
	"fmt"

	logdetails "github.com/RayleaBot/RayleaBot/server/internal/logging/details"
)

type spoolRecord struct {
	BootID    string         `json:"boot_id,omitempty"`
	LogID     string         `json:"log_id"`
	Timestamp string         `json:"timestamp"`
	Level     string         `json:"level"`
	Source    string         `json:"source"`
	Message   string         `json:"message"`
	Protocol  string         `json:"protocol,omitempty"`
	PluginID  string         `json:"plugin_id,omitempty"`
	RequestID string         `json:"request_id,omitempty"`
	Details   map[string]any `json:"details,omitempty"`
}

func spoolRecordFromSummary(summary Summary) spoolRecord {
	normalized := NormalizeSummary(summary)
	return spoolRecord{
		BootID:    normalized.BootID,
		LogID:     normalized.LogID,
		Timestamp: normalized.Timestamp,
		Level:     normalized.Level,
		Source:    normalized.Source,
		Message:   normalized.Message,
		Protocol:  normalized.Protocol,
		PluginID:  normalized.PluginID,
		RequestID: normalized.RequestID,
		Details:   logdetails.CloneMap(normalized.Details),
	}
}

func decodeSpoolRecord(line []byte) (Summary, error) {
	var record spoolRecord
	if err := json.Unmarshal(bytes.TrimSpace(line), &record); err != nil {
		return Summary{}, fmt.Errorf("decode spool record: %w", err)
	}
	return NormalizeSummary(Summary{
		BootID:    record.BootID,
		LogID:     record.LogID,
		Timestamp: record.Timestamp,
		Level:     record.Level,
		Source:    record.Source,
		Message:   record.Message,
		Protocol:  record.Protocol,
		PluginID:  record.PluginID,
		RequestID: record.RequestID,
		Details:   logdetails.CloneMap(record.Details),
	}), nil
}
