package task

import (
	"strconv"
	"sync"
	"time"

	"github.com/hashicorp/nomad/plugins/drivers"
	"github.com/shoenig/nomad-pledge/pkg/pledge"
	"github.com/shoenig/nomad-pledge/pkg/resources"
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

func RecreateHandle(runner pledge.Exec, config *drivers.TaskConfig, started time.Time) *Handle {
	clock := libtime.SystemClock()
	return &Handle{
		pid:     runner.PID(),
		runner:  runner,
		config:  config,
		state:   drivers.TaskStateUnknown,
		clock:   clock,
		started: started,
		result:  new(drivers.ExitResult),
	}
}

func (h *Handle) Stats() resources.Utilization {
	h.lock.RLock()
	defer h.lock.RUnlock()
	return h.runner.Stats()
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

	h.result.ExitCode = h.runner.Result()
	h.completed = h.clock.Now()
	h.state = drivers.TaskStateExited
}

func (h *Handle) Signal(s string) error {
	return h.runner.Signal(s)
}

func (h *Handle) Stop(signal string, timeout time.Duration) error {
	return h.runner.Stop(signal, timeout)
}
