package manager

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

func TestHelperProcessRuntime(t *testing.T) {
	if os.Getenv("RAYLEABOT_RUNTIME_HELPER") != "1" {
		return
	}

	runHelperProcessRuntime(
		os.Getenv("RAYLEABOT_RUNTIME_SCENARIO"),
		os.Getenv("RAYLEABOT_RUNTIME_RECORD"),
		bufio.NewScanner(os.Stdin),
	)
}

func runHelperProcessRuntime(scenario string, recordPath string, scanner *bufio.Scanner) {
	if runHelperProcessRuntimePart1(scenario, recordPath, scanner) {
		return
	}
	if runHelperProcessRuntimePart2(scenario, recordPath, scanner) {
		return
	}
	os.Exit(5)
}

func recordFrame(path string, line []byte) {
	if path == "" {
		return
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(6)
	}
	defer file.Close()

	if _, err := file.Write(append(line, '\n')); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(7)
	}
}

func writeHelperFrame(frame map[string]any) {
	encoded, err := json.Marshal(frame)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(8)
	}
	fmt.Printf("%s\n", encoded)
}

func helperReadFrame(scanner *bufio.Scanner, code int) map[string]any {
	if !scanner.Scan() {
		os.Exit(code)
	}
	line := append([]byte(nil), scanner.Bytes()...)
	var frame map[string]any
	if err := json.Unmarshal(line, &frame); err != nil {
		os.Exit(code + 100)
	}
	return frame
}

func helperExpectFrameType(scanner *bufio.Scanner, requestID string, frameType string, code int) map[string]any {
	frame := helperReadFrame(scanner, code)
	if frame["request_id"] != requestID || frame["type"] != frameType {
		os.Exit(code + 200)
	}
	return frame
}

func helperConsumeShutdown(scanner *bufio.Scanner, code int) {
	for scanner.Scan() {
		line := append([]byte(nil), scanner.Bytes()...)
		var frame map[string]any
		if err := json.Unmarshal(line, &frame); err != nil {
			os.Exit(code + 100)
		}
		if frame["type"] == "shutdown" {
			os.Exit(0)
		}
	}
	os.Exit(0)
}
