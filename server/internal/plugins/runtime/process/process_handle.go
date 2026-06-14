package process

import (
	"bufio"
	"io"
	"os/exec"
	"sync"
	"time"
)

type Spec struct {
	PluginID             string
	InitTimeout          time.Duration
	InitMaxTotal         time.Duration
	EventTimeout         time.Duration
	ShutdownGrace        time.Duration
	EffectiveConcurrency int
}

type Handle struct {
	Cmd    *exec.Cmd
	Stdin  io.WriteCloser
	Stdout *bufio.Reader
	Spec   Spec

	writeMu sync.Mutex
	done    chan struct{}
	exitMu  sync.RWMutex
	exitErr error
}

func NewHandle(cmd *exec.Cmd, stdin io.WriteCloser, stdout *bufio.Reader, spec Spec) *Handle {
	return &Handle{
		Cmd:    cmd,
		Stdin:  stdin,
		Stdout: stdout,
		Spec:   spec,
		done:   make(chan struct{}),
	}
}

func (h *Handle) Done() <-chan struct{} {
	if h == nil {
		closed := make(chan struct{})
		close(closed)
		return closed
	}
	return h.done
}

func (h *Handle) SetExit(err error) {
	h.exitMu.Lock()
	defer h.exitMu.Unlock()

	h.exitErr = err
	close(h.done)
}

func (h *Handle) ExitResult() (error, bool) {
	select {
	case <-h.done:
		h.exitMu.RLock()
		defer h.exitMu.RUnlock()
		return h.exitErr, true
	default:
		return nil, false
	}
}

func (h *Handle) Watch() {
	if h == nil || h.Cmd == nil {
		return
	}
	h.SetExit(h.Cmd.Wait())
}
