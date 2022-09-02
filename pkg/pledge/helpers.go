package pledge

import (
	"strconv"
	"time"
)

// currentPIDs returns the number of live processes in the cgroup.
func (e *exe) currentPIDs() int {
	s, err := e.readCG("pids.current")
	if err != nil {
		return -1
	}
	if s == "" {
		return 0
	}
	i, err := strconv.Atoi(s)
	if err != nil {
		return -1
	}
	return i
}

// blockPIDs blocks until there are no more live processes in the cgroup, and returns true
// if the timeout is exceeded or an error occurs.
func (e *exe) blockPIDs(timeout time.Duration) bool {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	abort := time.After(timeout)

	for {
		select {
		case <-ticker.C:
			count := e.currentPIDs()
			switch count {
			case 0:
				// processes are no longer running
				return false
			case -1:
				// failed to read cgroups file, issue force kill
				return true
			default:
				// processes are still running, wait longer
			}
		case <-abort:
			// timeout exceeded, issue force kill
			return true
		}
	}
}
