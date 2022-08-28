package resources

import (
	"strings"
	"syscall"
)

func ParseSignal(s string) syscall.Signal {
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
		return syscall.SIGABRT
	}
}
