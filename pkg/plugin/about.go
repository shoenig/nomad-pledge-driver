package plugin

import (
	"fmt"

	"github.com/hashicorp/nomad/plugins/base"
	"github.com/hashicorp/nomad/plugins/drivers"
	"github.com/hashicorp/nomad/plugins/shared/hclspec"
	"github.com/shoenig/nomad-pledge/pkg/pledge"
	"github.com/shoenig/nomad-pledge/pkg/resources"
)

const (
	// Name is the name of this plugin
	Name = "pledge"

	// Version enables the Client to identify the version of this plugin
	Version = "v0.1.0"

	// HandleVersion is the version of the task handle this plugin knows how
	// to decode
	HandleVersion = 1
)

// info describes the plugin to Nomad
var info = &base.PluginInfoResponse{
	Type:              base.PluginTypeDriver,
	PluginApiVersions: []string{drivers.ApiVersion010},
	PluginVersion:     Version,
	Name:              Name,
}

var driverConfigSpec = hclspec.NewObject(map[string]*hclspec.Spec{
	"pledge_executable": hclspec.NewAttr("pledge_executable", "string", true),
})

var taskConfigSpec = hclspec.NewObject(map[string]*hclspec.Spec{
	"command":    hclspec.NewAttr("command", "string", true),
	"args":       hclspec.NewAttr("args", "list(string)", false),
	"promises":   hclspec.NewAttr("promises", "string", false),
	"unveil":     hclspec.NewAttr("unveil", "list(string)", false),
	"importance": hclspec.NewAttr("importance", "string", false),
})

var capabilities = &drivers.Capabilities{
	SendSignals:         true,
	Exec:                false,
	FSIsolation:         drivers.FSIsolationNone,
	NetIsolationModes:   []drivers.NetIsolationMode{drivers.NetIsolationModeNone},
	MustInitiateNetwork: false,
	MountConfigs:        drivers.MountConfigSupportAll,
	RemoteTasks:         false,
}

// Config represents the pledge-driver plugin configuration that gets set in the
// Nomad client configuration file.
type Config struct {
	PledgeExecutable string `codec:"pledge_executable"`
}

// TaskConfig represents the pledge-driver task configuration that gets set in
// a Nomad job file.
type TaskConfig struct {
	Command    string   `codec:"command"`
	Args       []string `codec:"args"`
	Promises   string   `codec:"promises"`
	Unveil     []string `codec:"unveil"`
	Importance string   `codec:"importance"`
}

func parseOptions(driverTaskConfig *drivers.TaskConfig) (*pledge.Options, error) {
	var taskConfig TaskConfig
	if err := driverTaskConfig.DecodeDriverConfig(&taskConfig); err != nil {
		return nil, fmt.Errorf("failed to decode driver task config: %w", err)
	}
	importance, err := resources.ParseImportance(taskConfig.Importance)
	if err != nil {
		return nil, fmt.Errorf("failed to parse task importance: %w", err)
	}
	return &pledge.Options{
		Command:    taskConfig.Command,
		Arguments:  taskConfig.Args,
		Promises:   taskConfig.Promises,
		Unveil:     taskConfig.Unveil,
		Importance: importance,
	}, nil
}
