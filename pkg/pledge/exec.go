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
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/hashicorp/go-set/v2"
	"github.com/shoenig/nomad-pledge/pkg/resources"
	"github.com/shoenig/nomad-pledge/pkg/resources/process"
	"golang.org/x/sys/unix"
)

type Ctx = context.Context

type Environment struct {
	User      string            // user the command will run as
	Out       io.WriteCloser    // stdout handle
	Err       io.WriteCloser    // stderr handle
	Env       map[string]string // environment variables
	Dir       string            // task directory
	Cgroup    string            // task cgroup path
	Net       string            // allocation network namespace path
	Memory    uint64            // memory
	MemoryMax uint64            // memory_max
	Bandwidth uint64            // cpu / cores bandwidth (X/100_000)
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

func Recover(pid int, env *Environment) Exec {
	return &exe{
		pid:    pid,
		env:    env,
		opts:   nil, // necessary?
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

func (e *exe) openCG() (int, func(), error) {
	fd, err := unix.Open(e.env.Cgroup, unix.O_PATH, 0)
	cleanup := func() {
		_ = unix.Close(fd)
	}
	return fd, cleanup, err
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
	useless := set.From([]string{"LS_COLORS", "XAUTHORITY", "DISPLAY", "COLORTERM", "MAIL", "TMPDIR"})
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

func (e *exe) parameters(uid, gid uint32) []string {
	var result []string

	// start with nsenter if using bridge mode
	if net := e.env.Net; net != "" {
		result = append(
			result,
			"nsenter",
			"--no-fork",
			fmt.Sprintf("--net=%s", net),
			"--",
		)
	}

	// setup unshare for ipc, pid namespaces
	result = append(result,
		"unshare",
		"--ipc",
		"--pid",
		"--mount-proc",
		"--fork",
		"--kill-child=SIGKILL",
		"--setuid", strconv.Itoa(int(uid)),
		"--setgid", strconv.Itoa(int(gid)),
		"--",
	)

	// setup pledge invocation
	result = append(result, e.bin)

	// append the list of pledges
	if e.opts.Promises != "" {
		result = append(result, "-p", e.opts.Promises)
	}

	// append the list of unveils
	for _, u := range e.opts.Unveil {
		result = append(result, "-v", u)
	}

	// separate user command and args
	result = append(result, "--")

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

	// find our cgroup descriptor
	fd, cleanup, err := e.openCG()
	if err != nil {
		return fmt.Errorf("failed to open cgroup for descriptor")
	}

	// set resource constraints
	if err = e.constrain(); err != nil {
		return fmt.Errorf("failed to write resource constraints to cgroup: %w", err)
	}

	// a sandbox using nsenter, unshare, pledge, and our cgroup
	cmd := e.isolation(ctx, home, fd, uid, gid)
	if err = cmd.Start(); err != nil {
		return fmt.Errorf("failed to start command: %w", err)
	}

	// close cgroup descriptor
	cleanup()

	e.pid = cmd.Process.Pid
	e.waiter = process.WaitOnChild(cmd.Process)
	e.signal = process.Interrupts(cmd.Process.Pid)

	return nil
}

func (e *exe) isolation(ctx Ctx, home string, fd int, uid, gid uint32) *exec.Cmd {
	params := e.parameters(uid, gid)
	cmd := exec.CommandContext(ctx, params[0], params[1:]...)
	cmd.Stdout = e.env.Out
	cmd.Stderr = e.env.Err
	cmd.Env = flatten(e.env.User, home, e.env.Env)
	cmd.Dir = e.env.Dir
	cmd.SysProcAttr = &syscall.SysProcAttr{
		UseCgroupFD: true, // clone directly into cgroup
		CgroupFD:    fd,   // cgroup file descriptor
		Setpgid:     true, // ignore signals sent to nomad
	}
	return cmd
}

// set resource constraints via cgroups
func (e *exe) constrain() error {
	// set cpu bandwidth
	_ = e.writeCG("cpu.max", fmt.Sprintf("%d 100000", e.env.Bandwidth))

	// will want to set burst one day, but in coordination with nomad

	// set memory limits
	switch e.env.MemoryMax {
	case 0:
		_ = e.writeCG("memory.max", fmt.Sprintf("%d", e.env.Memory))
	default:
		_ = e.writeCG("memory.low", fmt.Sprintf("%d", e.env.Memory))
		_ = e.writeCG("memory.max", fmt.Sprintf("%d", e.env.MemoryMax))
	}

	// set CPU priority niceness
	_ = e.writeCG("cpu.weight.nice", strconv.Itoa(e.opts.Importance.Nice))

	return nil
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
	memCache := extractRe(memStatS, memCacheRe)

	cpuStatsS, _ := e.readCG("cpu.stat")
	usr, system, total := extractCPU(cpuStatsS)
	userPct, systemPct, totalPct := e.cpu.Percent(usr, system, total)

	specs, _ := resources.Get()
	ticks := (.01 * totalPct) * resources.Percent(specs.Ticks()/specs.Cores)

	return resources.Utilization{
		// memory stats
		Memory: uint64(memCurrent),
		Swap:   uint64(swapCurrent),
		Cache:  memCache,

		// cpu stats
		System:  systemPct,
		User:    userPct,
		Percent: totalPct,
		Ticks:   ticks,
	}
}

var (
	memCacheRe = regexp.MustCompile(`file\s+(\d+)`)
)

func extractRe(s string, re *regexp.Regexp) uint64 {
	matches := memCacheRe.FindStringSubmatch(s)
	if len(matches) != 2 {
		return 0
	}
	value, err := strconv.ParseInt(matches[1], 10, 64)
	if err != nil {
		return 0
	}
	return uint64(value)
}

func extractCPU(s string) (user, system, total resources.MicroSecond) {
	read := func(line string, i *resources.MicroSecond) {
		num := line[strings.Index(line, " ")+1:]
		v, _ := strconv.ParseInt(num, 10, 64)
		*i = resources.MicroSecond(v)
	}
	scanner := bufio.NewScanner(strings.NewReader(s))
	for scanner.Scan() {
		text := scanner.Text()
		switch {
		case strings.HasPrefix(text, "user_usec"):
			read(text, &user)
		case strings.HasPrefix(text, "system_usec"):
			read(text, &system)
		case strings.HasPrefix(text, "usage_usec"):
			read(text, &total)
		}
	}
	return
}
