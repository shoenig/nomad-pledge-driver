package task

import (
	"time"

	"github.com/hashicorp/nomad/plugins/drivers"
	"github.com/hashicorp/nomad/plugins/shared/structs"
)

// State is the runtime state encoded in the handle, returned
// to the Nomad client. Used to rebuild the task state and handler
// during recover.
type State struct {
	ReattachConfig *structs.ReattachConfig
	TaskConfig     *drivers.TaskConfig
	StartedAt      time.Time

	PID int
}
