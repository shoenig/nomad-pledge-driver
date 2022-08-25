package pledge

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/hashicorp/go-set"
	"oss.indeed.com/go/libtime"
)

type Ctx = context.Context

type Environment struct {
	User string
	Out  io.WriteCloser
	Err  io.WriteCloser
	Env  map[string]string
	Dir  string
}

type Options struct {
	Command   string
	Arguments []string
	Pledges   string
}

func (o *Options) String() string {
	return fmt.Sprintf("(%s, %v, %v)", o.Command, o.Arguments, o.Pledges)
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
	result := []string{e.bin}
	if e.opts.Pledges != "" {
		result = append(result, "-p", "'"+e.opts.Pledges+"'")
	}
	result = append(result, e.opts.Command)
	if len(e.opts.Arguments) > 0 {
		result = append(result, e.opts.Arguments...)
	}
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
	return e.cmd.Start()
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
	err := syscall.Kill(-e.cmd.Process.Pid, signal)
	go func() {
		timer, cancel := libtime.SafeTimer(timeout)
		defer cancel()
		<-timer.C
		if !e.cmd.ProcessState.Exited() {
			_ = syscall.Kill(-e.cmd.Process.Pid, syscall.SIGKILL)
		}

		_ = e.env.Out.Close()
		_ = e.env.Err.Close()
	}()
	return err
}
