package pledge

import (
	"bytes"
	"context"
	"io"
	"os"
	"testing"

	"github.com/shoenig/test/must"
)

const (
	pledgeEnvVar     = "PLEDGE_PATH"
	defaultPledgeBin = "/opt/bin/pledge.com"
)

const (
	binPledge    = "pledge"
	binPledgeCom = "pledge.com"
)

func lookupBin() string {
	if env := os.Getenv(pledgeEnvVar); env == "" {
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

func testEnv() (*Environment, *bytes.Buffer, *bytes.Buffer) {
	var out bytes.Buffer
	var err bytes.Buffer

	return &Environment{
		Out:  noopCloser(&out),
		Err:  noopCloser(&err),
		Env:  map[string]string{},
		Dir:  ".",
		User: "shoenig",
	}, &out, &err
}

func testOpts() *Options {
	return &Options{
		Command:   "echo",
		Arguments: []string{"hello", "world"},
	}
}

func TestExec_hello(t *testing.T) {
	env, stdout, stderr := testEnv()
	opts := testOpts()
	e := New(lookupBin(), env, opts)
	ctx := context.Background()
	must.NoError(t, e.Start(ctx))
	must.NoError(t, e.Wait())
	must.ContainsString(t, stdout.String(), "hello")
	must.Eq(t, "", stderr.String())
}
