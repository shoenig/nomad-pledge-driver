//go:build e2e

package e2e

import (
	"context"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/shoenig/test/must"
)

const timeout = 30 * time.Second

func pause() {
	if ci := os.Getenv("CI"); ci == "" {
		time.Sleep(500 * time.Millisecond)
	}
	time.Sleep(2 * time.Second)
}

func setup(t *testing.T) context.Context {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
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

	_ = run(t, ctx, "nomad", "job", "run", "../hack/env.hcl")
	statusOutput := run(t, ctx, "nomad", "job", "status", "env")

	alloc := allocFromJobStatus(t, statusOutput)
	containsAllocEnvRe := regexp.MustCompile(`NOMAD_SHORT_ALLOC_ID=` + alloc)

	logs := run(t, ctx, "nomad", "alloc", "logs", alloc)
	must.RegexMatch(t, containsAllocEnvRe, logs)
}

func TestBasic_cURL(t *testing.T) {
	ctx := setup(t)

	_ = run(t, ctx, "nomad", "job", "run", "../hack/curl.hcl")
	statusOutput := run(t, ctx, "nomad", "job", "status", "curl")

	alloc := allocFromJobStatus(t, statusOutput)
	logs := run(t, ctx, "nomad", "alloc", "logs", alloc)
	must.StrContains(t, logs, `<title>Example Domain</title>`)
}

func TestBasic_Sleep(t *testing.T) {
	ctx := setup(t)

	_ = run(t, ctx, "nomad", "job", "run", "../hack/sleep.hcl")

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

	_ = run(t, ctx, "nomad", "job", "run", "../hack/http.hcl")

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

	_ = run(t, ctx, "nomad", "job", "run", "../hack/passwd.hcl")

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
	// make sure job is complete
	time.Sleep(5 * time.Second)
	statusOutput := run(t, ctx, "nomad", "job", "status", "cgroup")

	alloc := allocFromJobStatus(t, statusOutput)
	cgroupRe := regexp.MustCompile(`0::/nomad\.slice/share.slice/` + alloc + `.+\.cat\.scope`)

	logs := run(t, ctx, "nomad", "alloc", "logs", alloc)
	must.RegexMatch(t, cgroupRe, logs)
}

func TestBasic_Bridge(t *testing.T) {
	ctx := setup(t)

	_ = run(t, ctx, "nomad", "job", "run", "../hack/bridge.hcl")

	serviceInfo := run(t, ctx, "nomad", "service", "info", "pybridge")
	addressRe := regexp.MustCompile(`([\d]+\.[\d]+.[\d]+\.[\d]+:[\d]+)`)

	m := addressRe.FindStringSubmatch(serviceInfo)
	must.SliceLen(t, 2, m, must.Sprint("expected to find address"))
	address := m[1]

	// curl service address
	curlOutput := run(t, ctx, "curl", "-s", address)
	must.StrContains(t, curlOutput, "<title>bridge mode</title>")

	// stop the service job
	_ = run(t, ctx, "nomad", "job", "stop", "-purge", "bridge")
}

func TestBasic_PIDNS(t *testing.T) {
	ctx := setup(t)

	_ = run(t, ctx, "nomad", "job", "run", "../hack/ps.hcl")
	logs := run(t, ctx, "nomad", "logs", "-job", "ps")
	lines := strings.Split(logs, "\n") // header and ps
	must.SliceLen(t, 2, lines, must.Sprintf("expected 2 lines, got %q", logs))
}

func TestBasic_Resources(t *testing.T) {
	ctx := setup(t)

	_ = run(t, ctx, "nomad", "job", "run", "../hack/resources.hcl")

	// make sure job is complete
	pause()

	t.Run("memory.max", func(t *testing.T) {
		logs := run(t, ctx, "nomad", "logs", "-job", "resources", "memory.max")
		v, err := strconv.Atoi(strings.TrimSpace(logs))
		must.NoError(t, err)
		must.Eq(t, 209_715_200, v)
	})

	t.Run("memory.max.oversub", func(t *testing.T) {
		logs := run(t, ctx, "nomad", "logs", "-job", "resources", "memory.max.oversub")
		v, err := strconv.Atoi(strings.TrimSpace(logs))
		must.NoError(t, err)
		must.Eq(t, 262_144_000, v)
	})

	t.Run("memory.low.oversub", func(t *testing.T) {
		logs := run(t, ctx, "nomad", "logs", "-job", "resources", "memory.low.oversub")
		v, err := strconv.Atoi(strings.TrimSpace(logs))
		must.NoError(t, err)
		must.Eq(t, 157_286_400, v)
	})

	t.Run("cpu.max", func(t *testing.T) {
		logs := run(t, ctx, "nomad", "logs", "-job", "resources", "cpu.max")
		s := strings.Fields(logs)[0]
		v, err := strconv.Atoi(s)
		must.NoError(t, err)
		// gave it cpu=1000 which is (proably) less than 1 core
		must.Less(t, 100_000, v)
	})

	t.Run("cpu.max.cores", func(t *testing.T) {
		logs := run(t, ctx, "nomad", "logs", "-job", "resources", "cpu.max.cores")
		s := strings.Fields(logs)[0]
		v, err := strconv.Atoi(s)
		must.NoError(t, err)
		must.Positive(t, v)
		// 1 core == 100000 bandwidth ...
		// TODO why did this get smaller with v1.7?
	})
}
