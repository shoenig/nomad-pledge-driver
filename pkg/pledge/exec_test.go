package pledge

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/user"
	"testing"

	"github.com/shoenig/nomad-pledge/pkg/resources"
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

type writeCloser struct {
	io.Writer
}

func (wc *writeCloser) Close() error {
	return nil
}

func noopCloser(w io.Writer) io.WriteCloser {
	return &writeCloser{w}
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
		Out:    noopCloser(&out),
		Err:    noopCloser(&err),
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
