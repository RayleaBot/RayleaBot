package render

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"testing"
	"time"
)

const browserCommandHelperEnvironment = "RAYLEABOT_RENDER_COMMAND_HELPER"

func TestBrowserCommandHelperProcess(t *testing.T) {
	if os.Getenv(browserCommandHelperEnvironment) != "1" {
		return
	}
	if _, err := fmt.Fprintln(os.Stdout, "ready"); err != nil {
		os.Exit(2)
	}
	// Stay alive until the parent either closes stdin or cancels the command.
	if _, err := io.Copy(io.Discard, os.Stdin); err != nil {
		os.Exit(3)
	}
	os.Exit(0)
}

func startBrowserCommandHelper(t *testing.T) (*exec.Cmd, context.CancelFunc, io.WriteCloser) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	command := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestBrowserCommandHelperProcess$")
	command.Env = append(os.Environ(), browserCommandHelperEnvironment+"=1")
	stdin, err := command.StdinPipe()
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	stdout, err := command.StdoutPipe()
	if err != nil {
		cancel()
		_ = stdin.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cancel()
		_ = stdin.Close()
		if command.Process != nil && command.ProcessState == nil {
			_ = command.Wait()
		}
	})
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	ready := make(chan error, 1)
	go func() {
		line, err := bufio.NewReader(stdout).ReadString('\n')
		if err == nil && line != "ready\n" {
			err = fmt.Errorf("unexpected helper readiness: %q", line)
		}
		ready <- err
	}()
	select {
	case err := <-ready:
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatalf("helper did not become ready: %v", ctx.Err())
	}
	return command, cancel, stdin
}

func TestChromiumRunnerCloseReapsCancelledCommand(t *testing.T) {
	command, cancel, _ := startBrowserCommandHelper(t)
	cancel()
	if command.Process == nil || command.ProcessState != nil {
		t.Fatal("expected a started command whose owner has not called Wait")
	}
	runner := &chromiumRunner{command: command, cancelBrowser: cancel}
	if err := runner.Close(); err != nil {
		t.Fatalf("close cancelled command: %v", err)
	}
	if command.ProcessState == nil {
		t.Fatal("Close returned without reaping the cancelled command")
	}
	if command.ProcessState.Success() {
		t.Fatal("cancelled helper unexpectedly exited successfully")
	}
	if err := runner.Close(); err != nil {
		t.Fatalf("repeat Close: %v", err)
	}
}

func TestChromiumRunnerCloseDoesNotWaitAgain(t *testing.T) {
	command, cancel, stdin := startBrowserCommandHelper(t)
	if err := stdin.Close(); err != nil {
		t.Fatal(err)
	}
	if err := command.Wait(); err != nil {
		t.Fatalf("helper did not exit normally: %v", err)
	}
	state := command.ProcessState
	runner := &chromiumRunner{command: command, cancelBrowser: cancel}
	for range 2 {
		if err := runner.Close(); err != nil {
			t.Fatalf("close an already reaped command: %v", err)
		}
	}
	if command.ProcessState != state {
		t.Fatal("Close changed the already collected exit status")
	}
}

func TestChromiumRunnerCloseUnstartedCommand(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	command := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestBrowserCommandHelperProcess$")
	runner := &chromiumRunner{command: command, cancelBrowser: cancel}
	for range 2 {
		if err := runner.Close(); err != nil {
			t.Fatalf("close an unstarted command: %v", err)
		}
	}
	if command.Process != nil || command.ProcessState != nil {
		t.Fatal("Close started or waited an unstarted command")
	}
}
