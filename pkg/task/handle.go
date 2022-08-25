package task

import (
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/hashicorp/nomad/plugins/drivers"
	"github.com/shoenig/nomad-pledge/pkg/pledge"
	"oss.indeed.com/go/libtime"
)

type Handle struct {
	lock sync.RWMutex

	runner    pledge.Exec
	config    *drivers.TaskConfig
	state     drivers.TaskState
	started   time.Time
	completed time.Time
	result    *drivers.ExitResult
	clock     libtime.Clock

	pid int
}

func NewHandle(runner pledge.Exec, config *drivers.TaskConfig) (*Handle, time.Time) {
	clock := libtime.SystemClock()
	now := clock.Now()
	return &Handle{
		pid:     runner.PID(),
		runner:  runner,
		config:  config,
		state:   drivers.TaskStateRunning,
		clock:   clock,
		started: now,
		result:  new(drivers.ExitResult),
	}, now
}

func (h *Handle) Status() *drivers.TaskStatus {
	h.lock.RLock()
	defer h.lock.RUnlock()

	return &drivers.TaskStatus{
		ID:          h.config.ID,
		Name:        h.config.Name,
		State:       h.state,
		StartedAt:   h.started,
		CompletedAt: h.completed,
		ExitResult:  h.result,
		DriverAttributes: map[string]string{
			"pid": strconv.Itoa(h.pid),
		},
	}
}

func (h *Handle) IsRunning() bool {
	h.lock.RLock()
	defer h.lock.RUnlock()
	return h.state == drivers.TaskStateRunning
}

func (h *Handle) Block() {
	err := h.runner.Wait()

	h.lock.Lock()
	defer h.lock.Unlock()

	if err != nil {
		h.result.Err = err
		h.state = drivers.TaskStateUnknown
		h.completed = h.clock.Now()
		return
	}

	h.state = drivers.TaskStateExited
	code, elapsed := h.runner.Result()
	h.completed = h.started.Add(elapsed)
	h.result.ExitCode = code
}

func (h *Handle) Signal(signal syscall.Signal) error {
	return h.runner.Signal(signal)
}

func (h *Handle) Stop(signal syscall.Signal, timeout time.Duration) error {
	return h.runner.Stop(signal, timeout)
}
