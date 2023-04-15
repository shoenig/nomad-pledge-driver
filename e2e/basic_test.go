//go:build e2e

package e2e

import (
	"context"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/shoenig/test/must"
)

func pause() {
	if ci := os.Getenv("CI"); ci == "" {
		time.Sleep(500 * time.Millisecond)
	}
	time.Sleep(2 * time.Second)
}

func setup(t *testing.T) context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() {
		run(t, ctx, "nomad", "system", "gc")
		cancel()
	})
	pause()
	return ctx
}

func run(t *testing.T, ctx context.Context, command string, args ...string) string {
	t.Log("RUN", "command:", command, "args:", args)
	cmd := exec.CommandContext(ctx, command, args...)
	b, err := cmd.CombinedOutput()
	output := strings.TrimSpace(string(b))
	if err != nil {
		t.Log("ERR:", err)
		t.Log("OUT:", output)
	}
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

func allocFromJobStatus(t *testing.T, s string) string {
	re := regexp.MustCompile(`([[:xdigit:]]+)\s+([[:xdigit:]]+)\s+group`)
	matches := re.FindStringSubmatch(s)
	must.Len(t, 3, matches, must.Sprint("regex results", matches))
	return matches[1]
}

func TestBasic_Env(t *testing.T) {
	ctx := setup(t)

	_ = run(t, ctx, "nomad", "job", "run", "../hack/env.nomad")
	statusOutput := run(t, ctx, "nomad", "job", "status", "env")

	alloc := allocFromJobStatus(t, statusOutput)
	containsAllocEnvRe := regexp.MustCompile(`NOMAD_SHORT_ALLOC_ID=` + alloc)

	logs := run(t, ctx, "nomad", "alloc", "logs", alloc)
	must.RegexMatch(t, containsAllocEnvRe, logs)
}

func TestBasic_cURL(t *testing.T) {
	ctx := setup(t)

	_ = run(t, ctx, "nomad", "job", "run", "../hack/curl.nomad")
	statusOutput := run(t, ctx, "nomad", "job", "status", "curl")

	alloc := allocFromJobStatus(t, statusOutput)
	logs := run(t, ctx, "nomad", "alloc", "logs", alloc)
	must.StrContains(t, logs, `<title>Example Domain</title>`)
}

func TestBasic_Sleep(t *testing.T) {
	ctx := setup(t)

	_ = run(t, ctx, "nomad", "job", "run", "../hack/sleep.nomad")

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

	_ = run(t, ctx, "nomad", "job", "run", "../hack/http.nomad")

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

	_ = run(t, ctx, "nomad", "job", "run", "../hack/passwd.nomad")

	// make sure job is failing
	time.Sleep(5 * time.Second)
	jobStatus := run(t, ctx, "nomad", "job", "status", "passwd")
	deadRe := regexp.MustCompile(`group\s+0\s+0\s+0\s+1\s+0\s+0\s+0`)
	must.RegexMatch(t, deadRe, jobStatus)

	// stop the job
	stopOutput := run(t, ctx, "nomad", "job", "stop", "-purge", "passwd")
	must.StrContains(t, stopOutput, `finished with status "complete"`)
}

func TestBasic_Cgroup(t *testing.T) {
	ctx := setup(t)

	_ = run(t, ctx, "nomad", "job", "run", "../hack/cgroup.hcl")
	statusOutput := run(t, ctx, "nomad", "job", "status", "cgroup")

	alloc := allocFromJobStatus(t, statusOutput)
	cgroupRe := regexp.MustCompile(`0::/nomad\.slice/` + alloc + `.+\.cat\.scope`)

	logs := run(t, ctx, "nomad", "alloc", "logs", alloc)
	must.RegexMatch(t, cgroupRe, logs)
}
