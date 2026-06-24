package config

import (
	"errors"
	"strings"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
)

type ValidationFieldDetail struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
	Hint   string `json:"hint,omitempty"`
}

func ValidationErrorDetails(err error) []ValidationFieldDetail {
	var validationErr *jsonschema.ValidationError
	if !errors.As(err, &validationErr) {
		return nil
	}

	var details []ValidationFieldDetail
	collectValidationFieldDetails(validationErr, &details)
	if len(details) == 0 {
		details = append(details, validationFieldDetail(validationErr))
	}
	return compactValidationFieldDetails(details)
}

func collectValidationFieldDetails(err *jsonschema.ValidationError, details *[]ValidationFieldDetail) {
	if err == nil {
		return
	}
	if len(err.Causes) == 0 {
		*details = append(*details, validationFieldDetail(err))
		return
	}
	for _, cause := range err.Causes {
		collectValidationFieldDetails(cause, details)
	}
}

func validationFieldDetail(err *jsonschema.ValidationError) ValidationFieldDetail {
	path := strings.Join(err.InstanceLocation, ".")
	if path == "" {
		path = "$"
	}
	return ValidationFieldDetail{
		Path:   path,
		Reason: err.Error(),
		Hint:   validationFieldHint(path),
	}
}

func validationFieldHint(path string) string {
	switch {
	case strings.HasSuffix(path, ".url"):
		return "请填写该字段支持的 URL 格式。"
	case strings.HasSuffix(path, ".access_token"):
		return "请填写字符串；留空表示不使用 token。"
	default:
		return "请检查该字段的类型、范围和格式。"
	}
}

func compactValidationFieldDetails(details []ValidationFieldDetail) []ValidationFieldDetail {
	seen := make(map[string]bool, len(details))
	compacted := make([]ValidationFieldDetail, 0, len(details))
	for _, detail := range details {
		key := detail.Path + "\x00" + detail.Reason
		if seen[key] {
			continue
		}
		seen[key] = true
		compacted = append(compacted, detail)
	}
	return compacted
}
