package pledge

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/hashicorp/go-set"
	"github.com/shoenig/nomad-pledge/pkg/resources"
	"github.com/shoenig/nomad-pledge/pkg/resources/process"
)

type Ctx = context.Context

type Environment struct {
	User   string
	Out    io.WriteCloser
	Err    io.WriteCloser
	Env    map[string]string
	Dir    string
	Cgroup string
}

func (o *Options) String() string {
	return fmt.Sprintf("(%s, %v, %s, %v, %s)", o.Command, o.Arguments, o.Promises, o.Unveil, o.Importance)
}

func New(bin string, env *Environment, opts *Options) Exec {
	return &exe{
		bin:  bin,
		env:  env,
		opts: opts,
		cpu:  new(resources.TrackCPU),
	}
}

func Recover(pid int) Exec {
	return &exe{
		pid:    pid,
		waiter: process.WaitOnOrphan(pid),
		signal: process.Interrupts(pid),
		cpu:    new(resources.TrackCPU),
	}
}

type Exec interface {
	// Start the process.
	Start(ctx Ctx) error

	// PID returns the process ID associated with exec.
	//
	// Must be called after Start.
	PID() int

	// Wait on the process.
	//
	// Must be called after Start.
	Wait() error

	//Stats returns current resource utilization.
	//
	// Must be called after Start.
	Stats() resources.Utilization

	// Signal the process.
	//
	// Must be called after Start.
	Signal(string) error

	// Stop the process.
	//
	// Must be called after Start.
	Stop(string, time.Duration) error

	// Result of the process after completion.
	//
	// Must be called after Wait.
	Result() int // exit code
}

// exe is the interface we create over a process
//
// We must always be able to re-create the exe object
// after an agent/plugin restart, given only a PID.
type exe struct {
	// pledge executable
	bin string

	// comes from task config
	env  *Environment
	opts *Options

	// comes from runtime
	pid    int
	cpu    *resources.TrackCPU
	waiter process.Waiter
	signal process.Signaler
	code   int
}

// lookup returns the uid, gid, and home directory of the given user.
func lookup(name string) (uint32, uint32, string, error) {
	u, err := user.Lookup(name)
	if err != nil {
		return 0, 0, "", fmt.Errorf("failed to find user %q: %w", name, err)
	}

	uid, err := strconv.ParseInt(u.Uid, 10, 32)
	if err != nil {
		return 0, 0, "", fmt.Errorf("failed to decode uid for user %q: %w", name, err)
	}

	gid, err := strconv.ParseInt(u.Gid, 10, 32)
	if err != nil {
		return 0, 0, "", fmt.Errorf("failed to decode gid for user %q: %w", name, err)
	}

	return uint32(uid), uint32(gid), u.HomeDir, nil
}

func (e *exe) PID() int {
	return e.pid
}

func (e *exe) readCG(file string) (string, error) {
	file = filepath.Join(e.env.Cgroup, file)
	b, err := os.ReadFile(file)
	return strings.TrimSpace(string(b)), err
}

func (e *exe) writeCG(file, content string) error {
	file = filepath.Join(e.env.Cgroup, file)
	f, err := os.OpenFile(file, os.O_WRONLY, 0700)
	if err != nil {
		return fmt.Errorf("failed to open cgroup file: %w", err)
	}
	if _, err = io.Copy(f, strings.NewReader(content)); err != nil {
		return fmt.Errorf("failed to write pid to cgroup file: %w", err)
	}
	return f.Close()
}

func flatten(user, home string, env map[string]string) []string {
	useless := set.From[string]([]string{"LS_COLORS", "XAUTHORITY", "DISPLAY", "COLORTERM", "MAIL", "TMPDIR"})
	result := make([]string, 0, len(env))
	for k, v := range env {
		switch {
		case k == "USER": // set correct $USER
			result = append(result, "USER="+user)
		case k == "HOME": // set correct $HOME
			result = append(result, "HOME="+home)
		case useless.Contains(k): // purge useless vars
			continue
		case v == "":
			result = append(result, k)
		default:
			result = append(result, k+"="+v)
		}
	}
	result = append(result, "TMPDIR="+os.TempDir())
	return result
}

func (e *exe) parameters() []string {
	// start with the pledge executable
	var result []string

	// append the list of pledges
	if e.opts.Promises != "" {
		result = append(result, "-p", e.opts.Promises)
	}

	// append the list of unveils
	for _, u := range e.opts.Unveil {
		result = append(result, "-v", u)
	}

	// append the user command
	result = append(result, e.opts.Command)
	if len(e.opts.Arguments) > 0 {
		result = append(result, e.opts.Arguments...)
	}

	// craft complete result
	return result
}

// prepare will simply run the pledge binary with no arguments - causing it
// to create the underlying .ape and sandbox.so files in the tmp directory
// specified. This is a workaround for some weird issue where creating these
// files does not work while in a cgroup, as is the case during normal start.
func (e *exe) prepare(uid, gid uint32) error {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, e.bin, "-h")
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid:    true, // ignore signals sent to nomad
		Credential: &syscall.Credential{Uid: uid, Gid: gid},
	}
	cmd.Env = []string{fmt.Sprintf("TMPDIR=%s", os.TempDir())}
	return cmd.Run()
}

func (e *exe) Start(ctx Ctx) error {
	uid, gid, home, err := lookup(e.env.User)
	if err != nil {
		return fmt.Errorf("failed to start command without user: %w", err)
	}

	// prepare sandbox.so library before launching real command in cgroup
	if err = e.prepare(uid, gid); err != nil {
		return fmt.Errorf("failed to prestart command: %w", err)
	}

	params := e.parameters()
	cmd := exec.CommandContext(ctx, e.bin, params...)
	cmd.Stdout = e.env.Out
	cmd.Stderr = e.env.Err
	cmd.Env = flatten(e.env.User, home, e.env.Env)
	cmd.Dir = e.env.Dir
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid:    true, // ignore signals sent to nomad
		Credential: &syscall.Credential{Uid: uid, Gid: gid},
	}

	if err = cmd.Start(); err != nil {
		return fmt.Errorf("failed to start command: %w", err)
	}

	e.pid = cmd.Process.Pid
	e.waiter = process.WaitOnChild(cmd.Process)
	e.signal = process.Interrupts(cmd.Process.Pid)

	// Ideally we would fork a trusted helper, enter the cgroup ourselves, then
	// exec into the user subprocess. This is fine for now.
	return e.isolate()
}

// isolate this process to the cgroup for this task
func (e *exe) isolate() error {
	err := e.writeCG("cgroup.procs", strconv.Itoa(e.PID()))
	_ = e.writeCG("cpu.weight.nice", strconv.Itoa(e.opts.Importance.Nice))
	return err
}

func (e *exe) Wait() error {
	exit := e.waiter.Wait()
	e.code = exit.Code
	return exit.Err
}

func (e *exe) Result() int {
	return e.code
}

func (e *exe) Signal(signal string) error {
	return e.signal.Signal(signal)
}

func (e *exe) Stop(signal string, timeout time.Duration) error {
	// politely ask the group to terminate via user specified signal
	err := e.Signal(signal)
	if e.blockPIDs(timeout) {
		// no more mr. nice guy, kill the whole cgroup
		_ = e.writeCG("cgroup.kill", "1")
		_ = e.env.Out.Close()
		_ = e.env.Err.Close()
	}
	return err
}

func (e *exe) Stats() resources.Utilization {
	memCurrentS, _ := e.readCG("memory.current")
	memCurrent, _ := strconv.Atoi(memCurrentS)

	swapCurrentS, _ := e.readCG("memory.swap.current")
	swapCurrent, _ := strconv.Atoi(swapCurrentS)

	memStatS, _ := e.readCG("memory.stat")
	memCache := extract(memStatS)["file"]

	cpuStatsS, _ := e.readCG("cpu.stat")
	cpuStatsM := extract(cpuStatsS)

	userUsec := cpuStatsM["user_usec"]
	systemUsec := cpuStatsM["system_usec"]
	totalUsec := cpuStatsM["usage_usec"]
	userPct, systemPct, totalPct := e.cpu.Percent(userUsec, systemUsec, totalUsec)

	specs, _ := resources.Get()
	ticks := (.01 * totalPct) * float64(specs.Ticks()/specs.Cores)

	return resources.Utilization{
		// memory stats
		Memory: uint64(memCurrent),
		Swap:   uint64(swapCurrent),
		Cache:  memCache,

		// cpu stats
		System:  userPct,
		User:    systemPct,
		Percent: totalPct,
		Ticks:   ticks,
	}
}

// todo just regex the fields we want
// todo also get the throttle timings
func extract(content string) map[string]uint64 {
	m := make(map[string]uint64)
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		text := scanner.Text()
		fields := strings.Fields(text)
		if len(fields) == 2 {
			if value, err := strconv.Atoi(fields[1]); err == nil {
				m[fields[0]] = uint64(value)
			}
		}
	}
	return m
}
