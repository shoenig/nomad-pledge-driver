package plugin

import (
	"fmt"
	"strings"

	"github.com/hashicorp/go-set"
)

// Acceptable promise groups; see https://justine.lol/pledge/
var acceptable = set.From([]string{
	"stdio",     // allows exit_group, close, dup, dup2, dup3, fchdir, fstat, fsync, fdatasync, ftruncate, getdents, getegid, getrandom, geteuid, getgid, getgroups, getitimer, getpgid, getpgrp, getpid, getppid, getresgid, getresuid, getrlimit, getsid, wait4, gettimeofday, getuid, lseek, madvise, brk, arch_prctl, uname, set_tid_address, clock_getres, clock_gettime, clock_nanosleep, mmap (PROT_EXEC and weird flags aren't allowed), mprotect (PROT_EXEC isn't allowed), msync, munmap, nanosleep, pipe, pipe2, read, readv, pread, recv, poll, recvfrom, preadv, write, writev, pwrite, pwritev, select, send, sendto (only if addr is null), setitimer, shutdown, sigaction (but SIGSYS is forbidden), sigaltstack, sigprocmask, sigreturn, sigsuspend, umask, socketpair, ioctl(FIONREAD), ioctl(FIONBIO), ioctl(FIOCLEX), ioctl(FIONCLEX), fcntl(F_GETFD), fcntl(F_SETFD), fcntl(F_GETFL), fcntl(F_SETFL)
	"rpath",     // (read-only path ops) allows chdir, getcwd, open(O_RDONLY), openat(O_RDONLY), stat, fstat, lstat, fstatat, access, faccessat, readlink, readlinkat, statfs, fstatfs
	"wpath",     // (write path ops) allows getcwd, open(O_WRONLY), openat(O_WRONLY), stat, fstat, lstat, fstatat, access, faccessat, readlink, readlinkat, chmod, fchmod, fchmodat
	"cpath",     // (create path ops) allows open(O_CREAT), openat(O_CREAT), rename, renameat, renameat2, link, linkat, symlink, symlinkat, unlink, rmdir, unlinkat, mkdir, mkdirat
	"dpath",     // (create special path ops) allows mknod, mknodat, mkfifo
	"chown",     // (file ownership changes) allows chown, fchown, lchown, fchownat
	"flock",     // allows flock, fcntl(F_GETLK), fcntl(F_SETLK), fcntl(F_SETLKW)
	"tty",       // allows ioctl(TIOCGWINSZ), ioctl(TCGETS), ioctl(TCSETS), ioctl(TCSETSW), ioctl(TCSETSF)
	"recvfd",    // allows recvmsg(SCM_RIGHTS)
	"sendfd",    // allows sendmsg
	"fattr",     // allows chmod, fchmod, fchmodat, utime, utimes, futimens, utimensat
	"inet",      // allows socket(AF_INET), listen, bind, connect, accept, accept4, getpeername, getsockname, setsockopt, getsockopt, sendto
	"unix",      // allows socket(AF_UNIX), listen, bind, connect, accept, accept4, getpeername, getsockname, setsockopt, getsockopt
	"dns",       // allows socket(AF_INET), sendto, recvfrom, connect
	"proc",      // allows fork, vfork, kill, getpriority, setpriority, prlimit, setrlimit, setpgid, setsid, sched_getscheduler, sched_setscheduler, sched_get_priority_min, sched_get_priority_max, sched_get_param, sched_set_param
	"thread",    // allows clone, futex, and permits PROT_EXEC in mprotect
	"id",        // allows setuid, setreuid, setresuid, setgid, setregid, setresgid, setgroups, prlimit, setrlimit, getpriority, setpriority, setfsuid, setfsgid
	"exec",      // allows execve, execveat. If the executable in question needs a loader, then you'll need rpath and prot_exec too
	"prot_exec", // allows creating executable memory (dynamic / ape)
	"tmppath",   // allows unlink, unlinkat, and lstat. When this promise is used, certain paths will be automatically unveiled too, e.g. /tmp
	"vminfo",    // using this causes paths such as /proc/stat to be automatically unveiled (e.g. for htop)
})

func checkPromises(s string) (string, error) {
	wanted := set.From(strings.Fields(s))
	unknown := wanted.Difference(acceptable)
	list := strings.Join(unknown.List(), " ")
	switch len(list) {
	case 0:
		return list, nil
	default:
		return "", fmt.Errorf("rejecting promises [%s]", list)
	}
}
