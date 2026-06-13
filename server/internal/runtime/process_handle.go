package runtime

import (
	"bufio"
	"io"
	"os/exec"
	"sync"
)

type processHandle struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Reader
	spec   Spec

	writeMu sync.Mutex
	done    chan struct{}
	exitMu  sync.RWMutex
	exitErr error
}

func newProcessHandle(cmd *exec.Cmd, stdin io.WriteCloser, stdout *bufio.Reader, spec Spec) *processHandle {
	return &processHandle{
		cmd:    cmd,
		stdin:  stdin,
		stdout: stdout,
		spec:   spec,
		done:   make(chan struct{}),
	}
}

func (h *processHandle) setExit(err error) {
	h.exitMu.Lock()
	defer h.exitMu.Unlock()

	h.exitErr = err
	close(h.done)
}

func (h *processHandle) exitResult() (error, bool) {
	select {
	case <-h.done:
		h.exitMu.RLock()
		defer h.exitMu.RUnlock()
		return h.exitErr, true
	default:
		return nil, false
	}
}
