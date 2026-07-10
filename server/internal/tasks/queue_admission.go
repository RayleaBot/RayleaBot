package tasks

// QueueAdmission reserves bounded execution-queue capacity before a durable
// task record is created. A reservation is released when the worker dequeues
// the corresponding job, so queue-full rejections never leave pending tasks.
type QueueAdmission struct {
	slots chan struct{}
}

func NewQueueAdmission(capacity int) *QueueAdmission {
	if capacity < 1 {
		capacity = 1
	}
	return &QueueAdmission{slots: make(chan struct{}, capacity)}
}

func (q *QueueAdmission) TryAcquire() bool {
	if q == nil {
		return false
	}
	select {
	case q.slots <- struct{}{}:
		return true
	default:
		return false
	}
}

func (q *QueueAdmission) Release() {
	if q == nil {
		return
	}
	select {
	case <-q.slots:
	default:
	}
}
