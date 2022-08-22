package plugin

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/drivers/shared/eventer"
	"github.com/hashicorp/nomad/plugins/base"
	"github.com/hashicorp/nomad/plugins/drivers"
	"github.com/hashicorp/nomad/plugins/shared/hclspec"
	"github.com/hashicorp/nomad/plugins/shared/structs"
	"github.com/shoenig/nomad-pledge/pkg/pledge"
	"github.com/shoenig/nomad-pledge/pkg/signals"
	"github.com/shoenig/nomad-pledge/pkg/task"
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
	driverConfig *base.ClientDriverConfig

	// tasks is the in-memory datastore mapping IDs to handles
	tasks task.Store

	// ctx is used to coordinate shutdown across subsystems
	ctx context.Context

	// cancel is used to shutdown the plugin and its subsystems
	cancel context.CancelFunc

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
		logger: logger,
	}
}

func (p *PledgeDriver) PluginInfo() (*base.PluginInfoResponse, error) {
	p.logger.Trace("PluginInfo return it")
	return info, nil
}

func (p *PledgeDriver) ConfigSchema() (*hclspec.Spec, error) {
	p.logger.Trace("ConfigSchema return it")
	return driverConfigSpec, nil
}

func (p *PledgeDriver) SetConfig(c *base.Config) error {
	p.logger.Trace("SetConfig enter")
	var config Config
	if len(c.PluginConfig) > 0 {
		if err := base.MsgPackDecode(c.PluginConfig, &config); err != nil {
			return err
		}
	}

	// validate the plugin configuration

	p.config = &config
	if p.config.PledgeExecutable == "" {
		return fmt.Errorf("pledge_executable must be set")
	}

	return nil
}

func (p *PledgeDriver) TaskConfigSchema() (*hclspec.Spec, error) {
	p.logger.Trace("TaskConfigSchema return it")
	return taskConfigSpec, nil

}

func (p *PledgeDriver) Capabilities() (*drivers.Capabilities, error) {
	p.logger.Trace("Capabilities return it")
	return capabilities, nil
}

func (p *PledgeDriver) Fingerprint(ctx context.Context) (<-chan *drivers.Fingerprint, error) {
	p.logger.Trace("Fingerprint")
	ch := make(chan *drivers.Fingerprint)
	go p.fingerprint(ctx, ch)
	return ch, nil
}

func (p *PledgeDriver) fingerprint(ctx context.Context, ch chan<- *drivers.Fingerprint) {
	p.logger.Trace("fingerprint")
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

	abs, err := filepath.Abs(p.config.PledgeExecutable)
	if err != nil {
		return &drivers.Fingerprint{
			Health:            drivers.HealthStateUndetected,
			HealthDescription: fmt.Sprintf("failed to detect absolute path of pledge executable: %w", err),
		}
	}

	if _, err = os.Stat(abs); err != nil {
		if os.IsNotExist(err) {
			healthState = drivers.HealthStateUndetected
			healthDescription = "pledge executable not found"
		} else {
			healthState = drivers.HealthStateUnhealthy
			healthDescription = "failed to stat pledge executable"
		}
	}

	// detect unveil support

	return &drivers.Fingerprint{
		Health:            healthState,
		HealthDescription: healthDescription,
		Attributes: map[string]*structs.Attribute{
			"driver.pledge.abs": structs.NewStringAttribute(abs),
			"driver.pledge.os":  structs.NewStringAttribute(runtime.GOOS),
		},
	}
}

func open(stdout, stderr string) (io.WriteCloser, io.WriteCloser, error) {
	a, err := os.OpenFile(stdout, unix.O_WRONLY, os.ModeNamedPipe)
	if err != nil {
		return nil, nil, err
	}
	b, err := os.OpenFile(stdout, unix.O_WRONLY, os.ModeNamedPipe)
	if err != nil {
		return nil, nil, err
	}
	return a, b, nil
}

func (p *PledgeDriver) StartTask(config *drivers.TaskConfig) (*drivers.TaskHandle, *drivers.DriverNetwork, error) {
	if config.User == "" {
		config.User = "nobody"
		p.logger.Debug("no user set so using default", "name", "nobody")
	}

	if _, exists := p.tasks.Get(config.ID); exists {
		p.logger.Error("task with id already started", "id", config.ID)
		return nil, nil, fmt.Errorf("task with ID %s already started", config.ID)
	}

	p.logger.Trace("will decode driver config")
	var taskConfig TaskConfig
	if err := config.DecodeDriverConfig(&taskConfig); err != nil {
		p.logger.Error("failed to decode driver config", "error", err)
		return nil, nil, fmt.Errorf("failed to decode driver task config: %w", err)
	}

	handle := drivers.NewTaskHandle(HandleVersion)
	handle.Config = config

	p.logger.Trace("task io", "out", config.StdoutPath, "err", config.StderrPath)
	stdout, stderr, err := open(config.StdoutPath, config.StderrPath)
	if err != nil {
		p.logger.Error("failed to open log files", "error", err)
		return nil, nil, fmt.Errorf("failed to open log file(s): %w", err)
	}

	// create the environment for pledge
	env := &pledge.Environment{
		Out:  stdout,
		Err:  stderr,
		Env:  config.Env,
		Dir:  config.TaskDir().Dir,
		User: config.User,
	}

	opts := &pledge.Options{
		Command:   taskConfig.Command,
		Arguments: taskConfig.Args,
		Pledges:   taskConfig.Pledges,
	}
	p.logger.Trace("pledge runner", "cmd", opts.Command, "args", opts.Arguments, "pledges", opts.Pledges)

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

	p.logger.Trace("handle will set driver state")
	if err = handle.SetDriverState(state); err != nil {
		return nil, nil, fmt.Errorf("failed to set driver state: %w", err)
	}

	p.logger.Trace("tasks will set config for handle", "id", config.ID)
	p.tasks.Set(config.ID, h)

	return handle, nil, nil
}

// RecoverTask will re-create the in-memory state of a task from a TaskHandle
// coming from Nomad.
func (p *PledgeDriver) RecoverTask(handle *drivers.TaskHandle) error {
	p.logger.Trace("RecoverTask enter")
	if handle == nil {
		return errors.New("failed to recover task, handle is nil")
	}

	if _, exists := p.tasks.Get(handle.Config.ID); exists {
		return nil // nothing to do
	}

	var taskState task.State
	if err := handle.GetDriverState(&taskState); err != nil {
		return fmt.Errorf("failed to decode task state: %w", err)
	}

	var taskConfig TaskConfig
	if err := taskState.TaskConfig.DecodeDriverConfig(&taskConfig); err != nil {
		return fmt.Errorf("failed to decode task config: %w", err)
	}

	// implement logic to recover task ...
	// without the executor indirection, we need

	panic("ehh not finished yet")
}

func (p *PledgeDriver) WaitTask(ctx context.Context, taskID string) (<-chan *drivers.ExitResult, error) {
	p.logger.Trace("WaitTask enter")

	handle, exists := p.tasks.Get(taskID)
	if !exists {
		return nil, fmt.Errorf("task does not exist: %s", taskID)
	}

	ch := make(chan *drivers.ExitResult)
	go func() {
		// todo: able to cancel ?
		p.logger.Debug("WaitTask start Block")
		handle.Block()
		p.logger.Debug("WaitTask done Block")
		result := handle.Status()
		ch <- result.ExitResult
	}()
	return ch, nil
}

func (p *PledgeDriver) StopTask(taskID string, timeout time.Duration, signal string) error {
	p.logger.Trace("StopTask enter", "id", taskID, "timeout", timeout, "signal", signal)

	// todo: respect timeout

	h, exists := p.tasks.Get(taskID)
	if !exists {
		return nil
	}
	return h.Signal(signals.From(signal))
}

func (p *PledgeDriver) DestroyTask(taskID string, force bool) error {
	p.logger.Trace("DestroyTask enter", "id", taskID, "force", force)

	// todo: respect force

	h, exists := p.tasks.Get(taskID)
	if !exists {
		return nil
	}
	return h.Signal(signals.From("sigkill"))
}

func (p *PledgeDriver) InspectTask(taskID string) (*drivers.TaskStatus, error) {
	p.logger.Trace("InspectTask enter")

	// todo
	return nil, fmt.Errorf("InspectTask not implemented")
}

func (p *PledgeDriver) TaskStats(ctx context.Context, taskID string, interval time.Duration) (<-chan *drivers.TaskResourceUsage, error) {
	p.logger.Trace("TaskStats enter")

	// todo: implement
	ch := make(chan *drivers.TaskResourceUsage)
	return ch, nil
}

func (p *PledgeDriver) TaskEvents(ctx context.Context) (<-chan *drivers.TaskEvent, error) {
	p.logger.Trace("TaskEvents enter")

	// todo: implement
	ch := make(chan *drivers.TaskEvent, 1)
	return ch, nil
}

func (p *PledgeDriver) SignalTask(taskID string, signal string) error {
	p.logger.Trace("SignalTask enter", "signal", signal)

	h, exists := p.tasks.Get(taskID)
	if !exists {
		return nil
	}
	return h.Signal(signals.From(signal))
}

func (p *PledgeDriver) ExecTask(taskID string, cmd []string, timeout time.Duration) (*drivers.ExecTaskResult, error) {
	p.logger.Trace("ExecTask enter")

	// todo
	return nil, fmt.Errorf("ExecTask not implemented")
}
