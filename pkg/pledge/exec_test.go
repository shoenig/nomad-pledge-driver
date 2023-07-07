package pledge

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/user"
	"testing"

	"github.com/shoenig/nomad-pledge/pkg/resources"
	"github.com/shoenig/nomad-pledge/pkg/util"
	"github.com/shoenig/test/must"
)

const (
	pledgeEnvVar     = "PLEDGE_PATH"
	defaultPledgeBin = "/opt/bin/pledge-1.8.com"
)

func lookupBin() string {
	if env := os.Getenv(pledgeEnvVar); env != "" {
		return env
	}
	return defaultPledgeBin
}

func whoami() string {
	u, err := user.Current()
	if err != nil {
		panic("unable to lookup current user")
	}
	return u.Username
}

func testEnv() (*Environment, *bytes.Buffer, *bytes.Buffer) {
	var out bytes.Buffer
	var err bytes.Buffer

	return &Environment{
		Out:    util.NullCloser(&out),
		Err:    util.NullCloser(&err),
		Env:    map[string]string{},
		Dir:    ".",
		User:   whoami(),
		Cgroup: fmt.Sprintf("/sys/fs/cgroup/user.slice/%s.%s.scope", "abc123", "test"),
	}, &out, &err
}

func testOpts() *Options {
	return &Options{
		Command:   "echo",
		Arguments: []string{"hello", "world"},
		Importance: &resources.Importance{
			Label: "low",
			Nice:  10,
		},
	}
}

func TestExec_hello(t *testing.T) {
	t.Skip("requires refactoring to use mocks")

	env, stdout, stderr := testEnv()
	t.Log("whoami user is", env.User)
	opts := testOpts()
	e := New(lookupBin(), env, opts)
	ctx := context.Background()
	must.NoError(t, e.Start(ctx))
	must.NoError(t, e.Wait())
	must.StrContains(t, stdout.String(), "hello")
	must.Eq(t, "", stderr.String())
}

func TestExec_extractCPU(t *testing.T) {
	content := `
usage_usec 20454080000
user_usec 16809820000
system_usec 3644260000
nr_periods 0
nr_throttled 0
throttled_usec 0
`

	usr, system, total := extractCPU(content)
	must.Eq(t, 16809820000, usr)
	must.Eq(t, 3644260000, system)
	must.Eq(t, 20454080000, total)
}

func TestExec_extractRe(t *testing.T) {
	content := `
anon 4295925760
file 12386787328
inactive_file 6338285568
active_file 5839114240
workingset_refault_file 0
workingset_activate_file 0
workingset_restore_file 0
`
	value := extractRe(content, memCacheRe)
	must.Eq(t, 12386787328, value)
}
