package pledge

import (
	"github.com/shoenig/nomad-pledge/pkg/resources"
)

type Options struct {
	Command    string
	Arguments  []string
	Promises   string
	Unveil     []string
	Importance *resources.Importance
}
