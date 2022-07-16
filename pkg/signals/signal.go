package signals

import (
	"os"
	"strings"
	"syscall"
)

func From(s string) os.Signal {
	switch strings.ToLower(s) {
	case "sighup":
		return syscall.SIGHUP
	case "sigint":
		return syscall.SIGINT
	case "sigquit":
		return syscall.SIGQUIT
	case "sigabrt":
		return syscall.SIGABRT
	case "sigkill":
		return syscall.SIGKILL
	case "sigusr1":
		return syscall.SIGUSR1
	case "sigusr2":
		return syscall.SIGUSR2
	case "sigterm":
		return syscall.SIGTERM
	default:
		return syscall.SIGILL
	}
}
