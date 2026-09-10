package desktop

import (
	"os"
	"os/exec"
	"testing"
	"time"
)

func TestOwnedShutdownHelper(t *testing.T) {
	if os.Getenv("RAYLEA_SHUTDOWN_TEST_HELPER") != "1" {
		return
	}
	time.Sleep(time.Minute)
	os.Exit(0)
}

func TestForceKillReapsOwnedProcessAndIsIdempotent(t *testing.T) {
	command := exec.Command(os.Args[0], "-test.run=^TestOwnedShutdownHelper$")
	command.Env = append(os.Environ(), "RAYLEA_SHUTDOWN_TEST_HELPER=1")
	configureChildProcess(command)
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = command.Process.Kill() })
	process := NewProcessController("")
	process.cmd = command
	go process.wait(command)
	if err := process.ForceKill(); err != nil {
		t.Fatal(err)
	}
	if process.IsRunning() || processAlive(command.Process.Pid) {
		t.Fatal("owned process remained after force kill")
	}
	if err := process.ForceKill(); err != nil {
		t.Fatalf("repeat force kill: %v", err)
	}
}
