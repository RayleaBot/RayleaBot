//go:build windows

package desktop

import (
	"os"
	"os/exec"
	"testing"
	"time"
)

const processSupervisionHelperEnv = "RAYLEA_TEST_PROCESS_SUPERVISION_HELPER"

func TestSuperviseProcessKillsChildWhenReleased(t *testing.T) {
	command := exec.Command(os.Args[0], "-test.run=^TestSuperviseProcessHelper$")
	command.Env = append(os.Environ(), processSupervisionHelperEnv+"=1")
	configureChildProcess(command)
	if err := command.Start(); err != nil {
		t.Fatalf("start helper process: %v", err)
	}
	release, err := superviseProcess(command.Process)
	if err != nil {
		_ = command.Process.Kill()
		_ = command.Wait()
		t.Fatalf("superviseProcess() error = %v", err)
	}
	release()

	done := make(chan error, 1)
	go func() {
		done <- command.Wait()
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		_ = command.Process.Kill()
		<-done
		t.Fatal("helper process survived after the supervision job was released")
	}
}

func TestSuperviseProcessHelper(t *testing.T) {
	if os.Getenv(processSupervisionHelperEnv) != "1" {
		return
	}
	time.Sleep(time.Minute)
}
