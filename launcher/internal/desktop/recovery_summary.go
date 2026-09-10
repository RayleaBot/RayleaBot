package desktop

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
)

func readRecoverySummary(logDirectory string) (*ServerRecoveryCompatibilitySummary, error) {
	file, err := os.Open(filepath.Join(logDirectory, "recovery-summary.json"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, &BoundaryError{Code: "launcher.recovery_summary_unreadable", Message: "无法读取本机恢复摘要。", Cause: err}
	}
	defer file.Close()
	summary, err := decodeServerResponse[ServerRecoveryCompatibilitySummary](file, "RecoveryCompatibilitySummary")
	if err != nil {
		return nil, &BoundaryError{Code: "launcher.recovery_summary_invalid", Message: "本机恢复摘要不符合正式约定，原文件已保留。", Cause: err}
	}
	return summary, nil
}

func parseRecoverySummary(payload []byte) *ServerRecoveryCompatibilitySummary {
	summary, err := decodeServerResponse[ServerRecoveryCompatibilitySummary](bytes.NewReader(payload), "RecoveryCompatibilitySummary")
	if err != nil {
		return nil
	}
	return summary
}
