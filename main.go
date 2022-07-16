package main

import (
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/plugins"
	"github.com/shoenig/nomad-pledge/pkg/plugin"
)

func main() {
	plugins.Serve(factory)
}

func factory(logger hclog.Logger) interface{} {
	return plugin.New(logger)
}
