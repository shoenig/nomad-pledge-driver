package pledge

import (
	"bufio"
	"context"
	"errors"
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
	"oss.indeed.com/go/libtime"
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

type Options struct {
	Command   string
	Arguments []string
	Pledges   string
	Unveil    []string
}

func (o *Options) String() string {
	return fmt.Sprintf("(%s, %v, %v, %v)", o.Command, o.Arguments, o.Pledges, o.Unveil)
}

func New(bin string, env *Environment, opts *Options) Exec {
	return &exe{
		bin:  bin,
		env:  env,
		opts: opts,
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
	Stats() Utilization

	// Signal the process.
	//
	// Must be called after Start.
	Signal(syscall.Signal) error

	// Stop the process.
	//
	// Must be called after Start.
	Stop(syscall.Signal, time.Duration) error

	// Result of the process after completion.
	//
	// Must be called after Wait.
	Result() (int, time.Duration)
}

type exe struct {
	bin  string // pledge executable
	env  *Environment
	opts *Options

	cmd *exec.Cmd
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

// ensureHome will create a pseudo home directory for user if home does not exist
func ensureHome(home, user string, uid, gid int) (string, error) {
	info, statErr := os.Stat(home)
	if errors.Is(statErr, os.ErrNotExist) {
		// e.g. service users will have a non-existent home directory
		home = "/tmp/pledge-" + user // if so, setup a home for them (probably belongs in /var/run)
		if info, statErr = os.Stat(home); errors.Is(statErr, os.ErrNotExist) {
			if mkErr := os.Mkdir(home, 0755); mkErr != nil {
				return "", mkErr
			} else {
				return home, os.Chown(home, uid, gid)
			}
		}
	} else if statErr != nil {
		return "", statErr
	} else if !info.IsDir() {
		return "", errors.New("home directory path is not a directory")
	}
	return home, nil
}

func (e *exe) PID() int {
	return e.cmd.Process.Pid
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
	useless := set.From[string]([]string{"LS_COLORS", "XAUTHORITY", "DISPLAY", "COLORTERM", "MAIL"})
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
	return result
}

func (e *exe) parameters() string {
	// start with the pledge executable
	result := []string{e.bin}

	// append the list of pledges
	if e.opts.Pledges != "" {
		result = append(result, "-p", "'"+e.opts.Pledges+"'")
	}

	// append the list of unveils
	for _, u := range e.opts.Unveil {
		result = append(result, "-v", "'"+u+"'")
	}

	// append the user command
	result = append(result, e.opts.Command)
	if len(e.opts.Arguments) > 0 {
		result = append(result, e.opts.Arguments...)
	}

	// craft complete result
	return strings.Join(result, " ")
}

func (e *exe) Start(ctx Ctx) error {
	uid, gid, home, err := lookup(e.env.User)
	if err != nil {
		return fmt.Errorf("failed to start command without user: %w", err)
	}

	home, err = ensureHome(home, e.env.User, int(uid), int(gid))
	if err != nil {
		return fmt.Errorf("failed to start command without home directory: %w", err)
	}

	params := e.parameters()
	e.cmd = exec.CommandContext(ctx, "/bin/sh", "-c", params)
	e.cmd.Stdout = e.env.Out
	e.cmd.Stderr = e.env.Err
	e.cmd.Env = flatten(e.env.User, home, e.env.Env)
	e.cmd.Dir = e.env.Dir
	e.cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid:    true, // ignore signals sent to nomad
		Credential: &syscall.Credential{Uid: uid, Gid: gid},
	}
	if err = e.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start command: %w", err)
	}

	// Ideally we would fork a trusted helper, enter the cgroup ourselves, then
	// exec into the user subprocess. This is fine for now.
	return e.isolate()
}

// isolate this process to the cgroup for this task
func (e *exe) isolate() error {
	return e.writeCG("cgroup.procs", strconv.Itoa(e.PID()))
}

func (e *exe) Wait() error {
	return e.cmd.Wait()
}

func (e *exe) Result() (int, time.Duration) {
	elapsed := e.cmd.ProcessState.UserTime()
	code := e.cmd.ProcessState.ExitCode()
	return code, elapsed
}

func (e *exe) Signal(signal syscall.Signal) error {
	return syscall.Kill(-e.cmd.Process.Pid, signal)
}

func (e *exe) Stop(signal syscall.Signal, timeout time.Duration) error {
	// politely ask the group to terminate via user specified signal
	err := e.Signal(signal)
	go func() {
		timer, cancel := libtime.SafeTimer(timeout)
		defer cancel()
		<-timer.C

		// no more mr. nice guy, kill the whole cgroup
		_ = e.writeCG("cgroup.kill", "1")
		_ = e.env.Out.Close()
		_ = e.env.Err.Close()
	}()
	return err
}

type Utilization struct {
	Memory uint64
	Swap   uint64
	Cache  uint64

	System          uint64
	User            uint64
	Percent         uint64
	ThrottlePeriods uint64
	ThrottleTime    uint64
	Ticks           uint64
}

func (e *exe) Stats() Utilization {
	memCurrentS, _ := e.readCG("memory.current")
	memCurrent, _ := strconv.Atoi(memCurrentS)

	swapCurrentS, _ := e.readCG("memory.swap.current")
	swapCurrent, _ := strconv.Atoi(swapCurrentS)

	// todo
	// cpuStats, _ := e.readCG("cpu.stat")

	return Utilization{
		Memory: uint64(memCurrent),
		Swap:   uint64(swapCurrent),
		Cache:  0, // ?

		System:  0,
		User:    0,
		Percent: 0,
	}
}

func extract(content string) map[string]uint64 {
	m := make(map[string]uint64)
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		if fields := strings.Fields(scanner.Text()); len(fields) == 2 {
			if value, err := strconv.Atoi(fields[1]); err != nil {
				m[fields[1]] = uint64(value)
			}
		}
	}
	return m
}

/*
    totalCPU := &drivers.CpuStats{
		SystemMode: systemModeCPU,
		UserMode:   userModeCPU,
		Percent:    percent,
		Measured:   ExecutorBasicMeasuredCpuStats,
		TotalTicks: systemCpuStats.TicksConsumed(percent),
	}

	totalMemory := &drivers.MemoryStats{
		RSS:      totalRSS,
		Swap:     totalSwap,
		Measured: ExecutorBasicMeasuredMemStats,
	}

	resourceUsage := drivers.ResourceUsage{
		MemoryStats: totalMemory,
		CpuStats:    totalCPU,
	}
	return &drivers.TaskResourceUsage{
		ResourceUsage: &resourceUsage,
		Timestamp:     ts,
		Pids:          pidStats,
	}

*/

/*
cgroup/nomad.slice/18df61c6-09fb-3b35-d2bd-03c0f17a9e8b.pyserver.scope
➜ ls
cgroup.controllers      cpu.max.burst          hugetlb.1GB.events        io.prio.class        memory.stat
cgroup.events           cpu.pressure           hugetlb.1GB.events.local  io.stat              memory.swap.current
cgroup.freeze           cpuset.cpus            hugetlb.1GB.max           io.weight            memory.swap.events
cgroup.kill             cpuset.cpus.effective  hugetlb.1GB.rsvd.current  memory.current       memory.swap.high
cgroup.max.depth        cpuset.cpus.partition  hugetlb.1GB.rsvd.max      memory.events        memory.swap.max
cgroup.max.descendants  cpuset.mems            hugetlb.2MB.current       memory.events.local  misc.current
cgroup.procs            cpuset.mems.effective  hugetlb.2MB.events        memory.high          misc.max
cgroup.stat             cpu.stat               hugetlb.2MB.events.local  memory.low           pids.current
cgroup.subtree_control  cpu.uclamp.max         hugetlb.2MB.max           memory.max           pids.events
cgroup.threads          cpu.uclamp.min         hugetlb.2MB.rsvd.current  memory.min           pids.max
cgroup.type             cpu.weight             hugetlb.2MB.rsvd.max      memory.numa_stat     rdma.current
cpu.idle                cpu.weight.nice        io.max                    memory.oom.group     rdma.max
cpu.max                 hugetlb.1GB.current    io.pressure               memory.pressure

cpu.stat)
usage_usec 40775984
user_usec 28691830
system_usec 12084153
nr_periods 0
nr_throttled 0
throttled_usec 0


*/
