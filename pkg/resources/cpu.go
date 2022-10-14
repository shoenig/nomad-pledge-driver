package resources

import (
	"os"
	"regexp"
	"strconv"
	"time"
)

type TrackCPU struct {
	prevTime   time.Time
	prevUser   MicroSecond
	prevSystem MicroSecond
	prevTotal  MicroSecond
}

// Percent returns the percentage of time spent in user, system, total CPU usage.
func (t *TrackCPU) Percent(user, system, total MicroSecond) (Percent, Percent, Percent) {
	now := time.Now()

	if t.prevUser == 0 && t.prevSystem == 0 {
		t.prevUser = user
		t.prevSystem = system
		t.prevTotal = total
		return 0.0, 0.0, 0.0
	}

	elapsed := now.Sub(t.prevTime).Microseconds()
	userPct := t.percent(t.prevUser, user, elapsed)
	systemPct := t.percent(t.prevSystem, system, elapsed)
	totalPct := t.percent(t.prevTotal, total, elapsed)
	t.prevUser = user
	t.prevSystem = system
	t.prevTotal = total
	t.prevTime = now
	return userPct, systemPct, totalPct
}

func (t *TrackCPU) percent(t1, t2 MicroSecond, elapsed int64) Percent {
	delta := t2 - t1
	if elapsed <= 0 || delta <= 0.0 {
		return 0.0
	}
	return Percent(float64(delta)/float64(elapsed)) * 100.0
}

type Specs struct {
	MHz   int
	Cores int
}

func (s *Specs) Ticks() int {
	return s.Cores * s.MHz
}

var mhzRe = regexp.MustCompile(`cpu MHz\s+:\s+(\d+)\.\d+`)

var processorRe = regexp.MustCompile(`processor\s+:\s+(\d+)`)

func Get() (*Specs, error) {
	b, err := os.ReadFile("/proc/cpuinfo")
	if err != nil {
		return nil, err
	}
	content := string(b)

	speed := 0
	results := mhzRe.FindAllStringSubmatch(content, -1)
	for _, result := range results {
		if mhz, _ := strconv.Atoi(result[1]); mhz > speed {
			speed = mhz
		}
	}

	cores := len(processorRe.FindAllStringSubmatch(content, -1))

	return &Specs{
		MHz:   speed,
		Cores: cores,
	}, nil
}
