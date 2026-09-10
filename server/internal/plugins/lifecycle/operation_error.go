package lifecycle

import (
	"errors"
	"strings"
)

// operationError keeps causes available to the owner while exposing only stage
// names and the actual transaction outcome to task consumers.
type operationError struct {
	state    string
	failures []string
	cause    error
}

func (e *operationError) Error() string {
	return e.state + ": " + strings.Join(e.failures, ", ")
}

func (e *operationError) Unwrap() error { return e.cause }

func (e *operationError) add(stage string, err error) {
	if err == nil {
		return
	}
	e.failures = append(e.failures, stage)
	e.cause = errors.Join(e.cause, err)
}

func (e *operationError) details() map[string]any {
	return map[string]any{
		"operation_state": e.state,
		"failures":        append([]string(nil), e.failures...),
	}
}

func (e *operationError) message() string {
	switch e.state {
	case "rolled_back":
		return "插件安装失败，已恢复安装前状态"
	case "rollback_failed":
		return "插件安装失败且回滚未完成，已保留安装工作目录，请检查失败阶段后恢复"
	case "committed":
		return "插件文件变更已完成，但后续处理失败，请检查失败阶段后重试"
	case "partial":
		return "插件目录删除未完成，请检查失败阶段后重试卸载"
	default:
		return "插件操作失败，安装目录未变更"
	}
}
