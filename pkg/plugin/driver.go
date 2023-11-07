package plugin

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/hashicorp/go-hclog"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/drivers/shared/eventer"
	"github.com/hashicorp/nomad/plugins/base"
	"github.com/hashicorp/nomad/plugins/drivers"
	"github.com/hashicorp/nomad/plugins/shared/hclspec"
	"github.com/hashicorp/nomad/plugins/shared/structs"
	"github.com/shoenig/nomad-pledge/pkg/pledge"
	"github.com/shoenig/nomad-pledge/pkg/resources"
	"github.com/shoenig/nomad-pledge/pkg/task"
	"github.com/shoenig/nomad-pledge/pkg/util"
	"golang.org/x/sys/unix"
	"oss.indeed.com/go/libtime"
)

type PledgeDriver struct {
	// events is used to handle multiplexing of TaskEvent calls such that
	// an event can be broadcast to all callers
	events *eventer.Eventer

	// config is the plugin configuration set by the SetConfig RPC
	config *Config

	// driverConfig is the driver-client configuration from Nomad
	// driverConfig *base.ClientDriverConfig

	// tasks is the in-memory datastore mapping IDs to handles
	tasks task.Store

	// ctx is used to coordinate shutdown across subsystems
	ctx context.Context

	// cancel is used to shutdown the plugin and its subsystems
	cancel context.CancelFunc

	// users looks up system users
	users util.Users

	// logger will log to the Nomad agent
	logger hclog.Logger
}

func New(log hclog.Logger) drivers.DriverPlugin {
	ctx, cancel := context.WithCancel(context.Background())
	logger := log.Named(Name)
	return &PledgeDriver{
		ctx:    ctx,
		cancel: cancel,
		events: eventer.NewEventer(ctx, logger),
		config: new(Config),
		tasks:  task.NewStore(),
		users:  util.NewUsers(),
		logger: logger,
	}
}

func (p *PledgeDriver) PluginInfo() (*base.PluginInfoResponse, error) {
	return info, nil
}

func (p *PledgeDriver) ConfigSchema() (*hclspec.Spec, error) {
	return driverConfigSpec, nil
}

func (p *PledgeDriver) SetConfig(c *base.Config) error {
	var config Config
	if len(c.PluginConfig) > 0 {
		if err := base.MsgPackDecode(c.PluginConfig, &config); err != nil {
			return err
		}
	}

	p.config = &config
	if p.config.PledgeExecutable == "" {
		return fmt.Errorf("pledge_executable must be set")
	}

	return nil
}

func (p *PledgeDriver) TaskConfigSchema() (*hclspec.Spec, error) {
	return taskConfigSpec, nil
}

func (p *PledgeDriver) Capabilities() (*drivers.Capabilities, error) {
	return capabilities, nil
}

func (p *PledgeDriver) Fingerprint(ctx context.Context) (<-chan *drivers.Fingerprint, error) {
	ch := make(chan *drivers.Fingerprint)
	go p.fingerprint(ctx, ch)
	return ch, nil
}

func (p *PledgeDriver) fingerprint(ctx context.Context, ch chan<- *drivers.Fingerprint) {
	defer close(ch)

	var timer, cancel = libtime.SafeTimer(0)
	defer cancel()

	for {
		p.logger.Trace("enter loop", "ctx exists", p.ctx == nil)
		select {
		case <-ctx.Done():
			return
		case <-p.ctx.Done():
			return
		case <-timer.C:
			ch <- p.doFingerprint()
			timer.Reset(30 * time.Second)
		}
	}
}

func (p *PledgeDriver) doFingerprint() *drivers.Fingerprint {
	healthState := drivers.HealthStateHealthy
	healthDescription := drivers.DriverHealthy

	// inspect pledge.com binary path
	abs, err := filepath.Abs(p.config.PledgeExecutable)
	if err != nil {
		return failure(drivers.HealthStateUndetected, "failed to detect absolute path of pledge executable")
	}

	// inspect pledge.com binary
	fi, err := os.Stat(abs)
	switch {
	case os.IsNotExist(err):
		return failure(drivers.HealthStateUndetected, "pledge executable not found")
	case err != nil:
		return failure(drivers.HealthStateUnhealthy, "failed to stat pledge executable")
	case fi.Mode()&0o111 == 0:
		return failure(drivers.HealthStateUnhealthy, "pledge binary is not executable")
	case !p.detect("pledge"):
		return failure(drivers.HealthStateUnhealthy, "kernel too old")
	case !p.detect("unveil"):
		return failure(drivers.HealthStateHealthy, "kernel landlock not enabled")
	}

	// inspect unshare binary
	uPath, uErr := exec.LookPath("unshare")
	switch {
	case os.IsNotExist(uErr):
		return failure(drivers.HealthStateUndetected, "unshare executable not found")
	case uErr != nil:
		return failure(drivers.HealthStateUnhealthy, "failed to find unshare executable")
	case uPath == "":
		return failure(drivers.HealthStateUndetected, "unshare executable does not exist")
	}

	// inspect nsenter binary
	nPath, nErr := exec.LookPath("nsenter")
	switch {
	case os.IsNotExist(nErr):
		return failure(drivers.HealthStateUndetected, "nsenter executable not found")
	case nErr != nil:
		return failure(drivers.HealthStateUnhealthy, "failed to find nsenter executable")
	case nPath == "":
		return failure(drivers.HealthStateUndetected, "nsenter executable does not exist")
	}

	// inspect cap_net_bind_service configuration
	// e.g. sudo setcap cap_net_bind_service+eip /opt/bin/pledge-1.8.com
	netCap := p.getcap("cap_net_bind_service")

	return &drivers.Fingerprint{
		Health:            healthState,
		HealthDescription: healthDescription,
		Attributes: map[string]*structs.Attribute{
			"driver.pledge.abs":          structs.NewStringAttribute(abs),
			"driver.pledge.os":           structs.NewStringAttribute(runtime.GOOS),
			"driver.pledge.cap.net_bind": structs.NewBoolAttribute(netCap),
		},
	}
}

func failure(state drivers.HealthState, desc string) *drivers.Fingerprint {
	return &drivers.Fingerprint{
		Health:            state,
		HealthDescription: desc,
	}
}

const timeout = 3 * time.Second

func (p *PledgeDriver) getcap(name string) bool {
	ctx, cancel := util.Timeout(timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "getcap", p.config.PledgeExecutable)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return false
	}

	exp := fmt.Sprintf("%s=eip", name) // todo: robustness
	return strings.Contains(string(out), exp)
}

func (p *PledgeDriver) detect(param string) bool {
	ctx, cancel := util.Timeout(timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "/bin/sh", "-c", strings.Join([]string{p.config.PledgeExecutable, "-T", param}, " "))
	_ = cmd.Run() // just check the exit code, non-zero means undetected
	return cmd.ProcessState.ExitCode() == 0
}

func open(stdout, stderr string) (io.WriteCloser, io.WriteCloser, error) {
	a, err := os.OpenFile(stdout, unix.O_WRONLY, os.ModeNamedPipe)
	if err != nil {
		return nil, nil, err
	}
	b, err := os.OpenFile(stderr, unix.O_WRONLY, os.ModeNamedPipe)
	if err != nil {
		return nil, nil, err
	}
	return a, b, nil
}

func netns(c *drivers.TaskConfig) string {
	switch {
	case c == nil:
		return ""
	case c.NetworkIsolation == nil:
		return ""
	case c.NetworkIsolation.Mode == drivers.NetIsolationModeGroup:
		return c.NetworkIsolation.Path
	default:
		return ""
	}
}

func (p *PledgeDriver) StartTask(config *drivers.TaskConfig) (*drivers.TaskHandle, *drivers.DriverNetwork, error) {
	if config.User == "" {
		current, _, _, err := p.users.Current()
		if err != nil {
			p.logger.Error("failed to lookup current user", "error", err)
			return nil, nil, err
		}
		config.User = current
		p.logger.Trace("no user set so using default", "name", current)
	}

	if _, exists := p.tasks.Get(config.ID); exists {
		p.logger.Error("task with id already started", "id", config.ID)
		return nil, nil, fmt.Errorf("task with ID %s already started", config.ID)
	}

	handle := drivers.NewTaskHandle(HandleVersion)
	handle.Config = config

	stdout, stderr, err := open(config.StdoutPath, config.StderrPath)
	if err != nil {
		p.logger.Error("failed to open log files", "error", err)
		return nil, nil, fmt.Errorf("failed to open log file(s): %w", err)
	}

	memory := uint64(config.Resources.NomadResources.Memory.MemoryMB) * 1024 * 1024
	memoryMax := uint64(config.Resources.NomadResources.Memory.MemoryMaxMB) * 1024 * 1024

	bandwidth, err := resources.Bandwidth(uint64(config.Resources.NomadResources.Cpu.CpuShares))
	if err != nil {
		p.logger.Error("failed to compute cpu bandwidth: %w", err)
		return nil, nil, fmt.Errorf("failed to compute cpu bandwidth: %w", err)
	}

	cpuset := config.Resources.LinuxResources.CpusetCpus
	p.logger.Trace("resources", "memory", memory, "memory_max", memoryMax, "compute", bandwidth, "cpuset", cpuset)

	// with cgroups v2 this is just the task cgroup
	cgroup := config.Resources.LinuxResources.CpusetCgroupPath

	// create the environment for pledge
	env := &pledge.Environment{
		Out:       stdout,
		Err:       stderr,
		Env:       config.Env,
		Dir:       config.TaskDir().Dir,
		User:      config.User,
		Cgroup:    cgroup,
		Net:       netns(config),
		Memory:    memory,
		MemoryMax: memoryMax,
		Bandwidth: bandwidth,
	}

	opts, err := parseOptions(config)
	if err != nil {
		return nil, nil, err
	}

	p.logger.Trace(
		"pledge runner",
		"cmd", opts.Command,
		"args", opts.Arguments,
		"promises", opts.Promises,
		"unveil", opts.Unveil,
		"importance", opts.Importance,
	)

	runner := pledge.New(p.config.PledgeExecutable, env, opts)
	if err = runner.Start(p.ctx); err != nil {
		return nil, nil, fmt.Errorf("failed to start command: %w", err)
	}

	h, started := task.NewHandle(runner, config)
	state := &task.State{
		PID:        runner.PID(),
		TaskConfig: config,
		StartedAt:  started,
	}

	if err = handle.SetDriverState(state); err != nil {
		return nil, nil, fmt.Errorf("failed to set driver state: %w", err)
	}

	p.logger.Trace("tasks will set config for handle", "id", config.ID)
	p.tasks.Set(config.ID, h)

	return handle, nil, nil
}

// RecoverTask will re-create the in-memory state of a task from a TaskHandle
// coming from Nomad. Hopefully this should never happen because the pledge driver
// runs independently from the Nomad Client process.
func (p *PledgeDriver) RecoverTask(handle *drivers.TaskHandle) error {
	if handle == nil {
		return errors.New("failed to recover task, handle is nil")
	}

	p.logger.Info("recovering task", "id", handle.Config.ID)

	if _, exists := p.tasks.Get(handle.Config.ID); exists {
		return nil // nothing to do
	}

	var taskState task.State
	if err := handle.GetDriverState(&taskState); err != nil {
		return fmt.Errorf("failed to decode task state: %w", err)
	}

	taskState.TaskConfig = handle.Config.Copy()

	// with cgroups v2 this is just the task cgroup
	cgroup := taskState.TaskConfig.Resources.LinuxResources.CpusetCgroupPath

	// re-create the environment for pledge
	env := &pledge.Environment{
		Out:    util.NullCloser(nil),
		Err:    util.NullCloser(nil),
		Env:    handle.Config.Env,
		Dir:    handle.Config.TaskDir().Dir,
		User:   handle.Config.User,
		Cgroup: cgroup,
	}

	runner := pledge.Recover(taskState.PID, env)
	recHandle := task.RecreateHandle(runner, taskState.TaskConfig, taskState.StartedAt)
	p.tasks.Set(taskState.TaskConfig.ID, recHandle)
	return nil
}

// WaitTask waits on the task to reach completion - whether by terminating
// gracefully and setting an exit code or by being rudely interrupted.
func (p *PledgeDriver) WaitTask(ctx context.Context, taskID string) (<-chan *drivers.ExitResult, error) {
	p.logger.Trace("waiting on task", "id", taskID)

	handle, exists := p.tasks.Get(taskID)
	if !exists {
		return nil, fmt.Errorf("task does not exist: %s", taskID)
	}

	ch := make(chan *drivers.ExitResult)
	go func() {
		// todo: able to cancel ?
		handle.Block()
		result := handle.Status()
		ch <- result.ExitResult
	}()
	return ch, nil
}

func (p *PledgeDriver) StopTask(taskID string, timeout time.Duration, signal string) error {
	p.logger.Debug("stop task", "id", taskID, "timeout", timeout, "signal", signal)

	if signal == "" {
		signal = "sigterm"
	}

	h, exists := p.tasks.Get(taskID)
	if !exists {
		return nil
	}
	return h.Stop(signal, timeout)
}

func (p *PledgeDriver) DestroyTask(taskID string, force bool) error {
	p.logger.Debug("destroy task", "id", taskID, "force", force)

	h, exists := p.tasks.Get(taskID)
	if !exists {
		return nil
	}

	var err error
	if h.IsRunning() {
		switch force {
		case false:
			err = errors.New("cannot destroy running task")
		case true:
			err = h.Stop("sigabrt", 100*time.Millisecond)
		}
	}

	p.tasks.Del(taskID)
	return err
}

func (p *PledgeDriver) InspectTask(taskID string) (*drivers.TaskStatus, error) {
	p.logger.Trace("InspectTask enter")

	// todo
	return nil, fmt.Errorf("InspectTask not implemented")
}

func (p *PledgeDriver) TaskStats(ctx context.Context, taskID string, interval time.Duration) (<-chan *drivers.TaskResourceUsage, error) {
	h, exists := p.tasks.Get(taskID)
	if !exists {
		return nil, nil
	}
	ch := make(chan *drivers.TaskResourceUsage)
	go p.stats(ctx, ch, interval, h)
	return ch, nil
}

func (p *PledgeDriver) stats(ctx context.Context, ch chan<- *drivers.TaskResourceUsage, interval time.Duration, h *task.Handle) {
	defer close(ch)
	ticks, stop := libtime.SafeTimer(interval)
	defer stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticks.C:
			ticks.Reset(interval)
		}

		usage := h.Stats()

		ch <- &drivers.TaskResourceUsage{
			ResourceUsage: &cstructs.ResourceUsage{
				MemoryStats: &cstructs.MemoryStats{
					Cache:    usage.Cache,
					Swap:     usage.Swap,
					Usage:    usage.Memory,
					Measured: []string{"Cache", "Swap", "Usage"},
				},
				CpuStats: &cstructs.CpuStats{
					UserMode:         float64(usage.User),
					SystemMode:       float64(usage.System),
					Percent:          float64(usage.Percent),
					TotalTicks:       float64(usage.Ticks),
					ThrottledPeriods: 0,
					ThrottledTime:    0,
					Measured:         []string{"System Mode", "User Mode", "Percent"},
				},
			},
			Timestamp: time.Now().UTC().UnixNano(),
			Pids:      nil,
		}
	}
}

func (p *PledgeDriver) TaskEvents(_ context.Context) (<-chan *drivers.TaskEvent, error) {
	// is there any use for this?
	// e.g.
	//  d.eventer.EmitEvent(&drivers.TaskEvent{
	//	  TaskID:    task.ID,
	//	  AllocID:   task.AllocID,
	//	  TaskName:  task.Name,
	//	  Timestamp: time.Now(),
	//	  Message:   "Downloading image",
	//	  Annotations: map[string]string{
	//	  		"image": dockerImageRef(repo, tag),
	//	  },
	//  })
	ch := make(chan *drivers.TaskEvent, 1)
	return ch, nil
}

func (p *PledgeDriver) SignalTask(taskID string, signal string) error {
	if signal == "" {
		return errors.New("signal must be set")
	}
	h, exists := p.tasks.Get(taskID)
	if !exists {
		return nil
	}
	return h.Signal(signal)
}

func (p *PledgeDriver) ExecTask(taskID string, cmd []string, timeout time.Duration) (*drivers.ExecTaskResult, error) {
	p.logger.Trace("ExecTask enter")

	// todo
	return nil, fmt.Errorf("ExecTask not implemented")
}
