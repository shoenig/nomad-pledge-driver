package process

import (
	"errors"
	"os"
	"syscall"
)

type Exit struct {
	Code      int
	Interrupt int
	Err       error
}

type Waiter interface {
	Wait() *Exit
}

func WaitOnChild(p *os.Process) Waiter {
	return &execWaiter{p: p}
}

type execWaiter struct {
	p *os.Process
}

func (w *execWaiter) Wait() *Exit {
	ps, err := w.p.Wait()
	status := ps.Sys().(syscall.WaitStatus)
	code := ps.ExitCode()
	if code < 0 {
		// just be cool
		code = int(status) + 128
	}
	return &Exit{
		Code:      code,
		Interrupt: int(status),
		Err:       err,
	}
}

func WaitOnOrphan(pid int) Waiter {
	return &pidWaiter{
		// todo
	}
}

type pidWaiter struct {
	//
}

func (w *pidWaiter) Wait() *Exit {
	return &Exit{
		Err: errors.New("not yet implemented"),
	}
}
