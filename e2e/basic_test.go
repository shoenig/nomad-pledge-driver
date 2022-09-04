package e2e

import (
	"context"
	"os/exec"
	"regexp"
	"strings"
	"testing"

	"github.com/shoenig/test/must"
)

func setup(t *testing.T) context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() {
		run(t, ctx, "nomad", "system", "gc")
		cancel()
	})
	return ctx
}

func run(t *testing.T, ctx context.Context, command string, args ...string) string {
	cmd := exec.CommandContext(ctx, command, args...)
	b, err := cmd.CombinedOutput()
	output := strings.TrimSpace(string(b))
	must.NoError(t, err, must.Sprint("output", output))
	return output
}

func TestBasic_Startup(t *testing.T) {
	ctx := setup(t)

	// can connect to nomad
	jobs := run(t, ctx, "nomad", "job", "status")
	must.Eq(t, "No running jobs", jobs)

	// pledge plugin is present and healthy
	status := run(t, ctx, "nomad", "node", "status", "-self", "-verbose")
	pledgeRe := regexp.MustCompile(`pledge\s+true\s+true\s+Healthy`)
	must.RegexMatch(t, pledgeRe, status)
}

func allocFromJobRun(t *testing.T, s string) string {
	re := regexp.MustCompile(`Allocation "([[:xdigit:]]+)" created:`)
	matches := re.FindStringSubmatch(s)
	must.Len(t, 2, matches, must.Sprint("output", s))
	return matches[1]
}

func TestBasic_Env(t *testing.T) {
	ctx := setup(t)

	runOutput := run(t, ctx, "nomad", "job", "run", "../hack/env.nomad")
	must.StrContains(t, runOutput, `finished with status "complete"`)

	alloc := allocFromJobRun(t, runOutput)
	containsAllocEnvRe := regexp.MustCompile(`NOMAD_SHORT_ALLOC_ID=` + alloc)

	logs := run(t, ctx, "nomad", "alloc", "logs", alloc)
	must.RegexMatch(t, containsAllocEnvRe, logs)
}

func TestBasic_cURL(t *testing.T) {
	ctx := setup(t)

	runOutput := run(t, ctx, "nomad", "job", "run", "../hack/curl.nomad")
	must.StrContains(t, runOutput, `finished with status "complete"`)

	alloc := allocFromJobRun(t, runOutput)
	logs := run(t, ctx, "nomad", "alloc", "logs", alloc)
	must.StrContains(t, logs, `<title>Example Domain</title>`)
}

func TestBasic_Sleep(t *testing.T) {
	ctx := setup(t)

	runOutput := run(t, ctx, "nomad", "job", "run", "../hack/sleep.nomad")
	must.StrContains(t, runOutput, `finished with status "complete"`)

	// no log output, make sure jbo is running
	jobStatus := run(t, ctx, "nomad", "job", "status", "sleep")
	runningRe := regexp.MustCompile(`Status\s+=\s+running`)
	must.RegexMatch(t, runningRe, jobStatus)

	// stop the job
	stopOutput := run(t, ctx, "nomad", "job", "stop", "sleep")
	must.StrContains(t, stopOutput, `finished with status "complete"`)

	// check job is stopped
	stopStatus := run(t, ctx, "nomad", "job", "status", "sleep")
	deadRe := regexp.MustCompile(`Status\s+=\s+dead\s+\(stopped\)`)
	must.RegexMatch(t, deadRe, stopStatus)
}

func TestBasic_HTTP(t *testing.T) {
	ctx := setup(t)

	runOutput := run(t, ctx, "nomad", "job", "run", "../hack/http.nomad")
	must.StrContains(t, runOutput, `finished with status "complete"`)

	// make sure job is running
	jobStatus := run(t, ctx, "nomad", "job", "status", "http")
	runningRe := regexp.MustCompile(`Status\s+=\s+running`)
	must.RegexMatch(t, runningRe, jobStatus)

	// curl localhost:8181
	curlOutput := run(t, ctx, "curl", "-s", "localhost:8181")
	must.StrContains(t, curlOutput, `<title>example</title>`)

	// stop the job
	stopOutput := run(t, ctx, "nomad", "job", "stop", "http")
	must.StrContains(t, stopOutput, `finished with status "complete"`)

	// check job is stopped
	stopStatus := run(t, ctx, "nomad", "job", "status", "http")
	stoppedRe := regexp.MustCompile(`Status\s+=\s+dead\s+\(stopped\)`)
	must.RegexMatch(t, stoppedRe, stopStatus)
}

func TestBasic_Passwd(t *testing.T) {
	ctx := setup(t)

	runOutput := run(t, ctx, "nomad", "job", "run", "../hack/passwd.nomad")
	must.StrContains(t, runOutput, `finished with status "complete"`)

	// make sure job is failing
	jobStatus := run(t, ctx, "nomad", "job", "status", "passwd")
	deadRe := regexp.MustCompile(`group\s+0\s+0\s+0\s+1\s+0\s+0\s+0`)
	must.RegexMatch(t, deadRe, jobStatus)

	// stop the job
	stopOutput := run(t, ctx, "nomad", "job", "stop", "-purge", "passwd")
	must.StrContains(t, stopOutput, `finished with status "complete"`)
}
