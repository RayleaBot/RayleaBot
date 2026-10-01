package desktop

import "sync"

const ResumeServiceFlag = "--resume-service"

// ServiceResume consumes the update handoff once, including a failed attempt.
// Initialization and the normal Start path retain ownership and startup checks.
type ServiceResume struct {
	requested bool
	once      sync.Once
	err       error
}

func NewServiceResume(args []string) *ServiceResume {
	resume := &ServiceResume{}
	for _, arg := range args {
		if arg == ResumeServiceFlag {
			resume.requested = true
		}
	}
	return resume
}

func (r *ServiceResume) AfterInitialize(service interface {
	GetSnapshot() (LauncherSnapshot, error)
	Start() error
}) error {
	r.once.Do(func() {
		if !r.requested {
			return
		}
		snapshot, err := service.GetSnapshot()
		if err != nil {
			r.err = err
			return
		}
		if snapshot.Launcher.ProcessOwnership != OwnershipNone || snapshot.Launcher.ProcessLifecycle != Stopped {
			return
		}
		r.err = service.Start()
	})
	return r.err
}
