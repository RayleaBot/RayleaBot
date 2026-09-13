package conversation

import (
	"context"
	"slices"
	"sync"
	"testing"
	"time"
)

// A ticket owns a FIFO position before it owns an execution permit. Closing
// ready wakes the worker for activation and cancellation; no polling is needed.
type ticket struct {
	message                  string
	ready                    chan struct{}
	done                     chan struct{}
	activateOnce, settleOnce sync.Once
	mu                       sync.Mutex
	canceled                 bool
}

func reserve(message string) *ticket {
	return &ticket{message: message, ready: make(chan struct{}), done: make(chan struct{})}
}
func (t *ticket) activate() { t.activateOnce.Do(func() { close(t.ready) }) }
func (t *ticket) settle()   { t.settleOnce.Do(func() { close(t.done) }) }
func (t *ticket) cancel() {
	t.mu.Lock()
	t.canceled = true
	t.mu.Unlock()
	t.activate()
	t.settle()
}
func (t *ticket) runnable() bool { t.mu.Lock(); defer t.mu.Unlock(); return !t.canceled }

func runLane(ctx context.Context, queue []*ticket, delivered chan<- string) {
	for _, item := range queue {
		select {
		case <-ctx.Done():
			for _, pending := range queue {
				pending.cancel()
			}
			return
		case <-item.ready:
		}
		if item.runnable() && ctx.Err() == nil {
			delivered <- item.message
		}
		item.settle()
	}
}
func receive[T any](t *testing.T, ch <-chan T) T {
	t.Helper()
	select {
	case value := <-ch:
		return value
	case <-time.After(2 * time.Second):
		t.Fatal("suspended ticket or missing wakeup")
		var zero T
		return zero
	}
}

func TestDesignReservedFIFOAndStopWakeup(t *testing.T) {
	for _, stop := range []bool{false, true} {
		ctx, cancel := context.WithCancel(context.Background())
		a, b := reserve("A"), reserve("B")
		out := make(chan string, 2)
		workerDone := make(chan struct{})
		go func() { defer close(workerDone); runLane(ctx, []*ticket{a, b}, out) }()
		b.activate()
		// B cannot cross A's inactive position, even though B's layer is ready.
		if stop {
			a.cancel()
		} else {
			a.activate()
		}
		receive(t, workerDone)
		close(out)
		var actual []string
		for value := range out {
			actual = append(actual, value)
		}
		want := []string{"A", "B"}
		if stop {
			want = []string{"B"}
		}
		if !slices.Equal(actual, want) {
			t.Fatalf("FIFO: %v want %v", actual, want)
		}
		for _, item := range []*ticket{a, b} {
			receive(t, item.done)
			item.cancel()
			item.settle()
		}
		cancel()
	}
}

func TestDesignStopReleasesEveryInactiveTicket(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	queue := []*ticket{reserve("A"), reserve("B"), reserve("C")}
	done := make(chan struct{})
	go func() { defer close(done); runLane(ctx, queue, make(chan string, 3)) }()
	cancel()
	receive(t, done)
	for _, item := range queue {
		receive(t, item.done)
	}
}

// FIFO edges increase admission sequence; same-message layer edges increase
// phase. Lexicographic (sequence, phase) is a strict rank, including when a
// reload reverses P/Q priorities for the next admitted message.
func TestDesignDependencyGraphIsAcyclic(t *testing.T) {
	for mask := 0; mask < 256; mask++ {
		type node struct{ sequence, phase, plugin int }
		var nodes []node
		for sequence := range 4 {
			first := (mask >> (sequence * 2)) & 1
			if mask&(1<<(sequence*2+1)) != 0 {
				nodes = append(nodes, node{sequence, 0, first})
			} else {
				nodes = append(nodes, node{sequence, 0, first}, node{sequence, 1, 1 - first})
			}
		}
		for _, a := range nodes {
			for _, b := range nodes {
				fifo := a.plugin == b.plugin && a.sequence < b.sequence
				layer := a.sequence == b.sequence && a.phase < b.phase
				if (fifo || layer) && !(a.sequence < b.sequence || a.sequence == b.sequence && a.phase < b.phase) {
					t.Fatalf("non-increasing dependency: %v -> %v", a, b)
				}
			}
		}
	}
}
