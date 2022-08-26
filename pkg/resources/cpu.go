package resources

import (
	"os"
	"regexp"
	"strconv"
	"time"
)

type TrackCPU struct {
	prevTime       time.Time
	prevUserUsec   uint64
	prevSystemUsec uint64
	prevTotalUsec  uint64
}

// Percent returns the percentage of time spent in user, system, total CPU usage.
func (t *TrackCPU) Percent(userUsec, systemUsec, totalUsec uint64) (float64, float64, float64) {
	now := time.Now()

	if t.prevUserUsec == 0 && t.prevSystemUsec == 0 {
		t.prevUserUsec = userUsec
		t.prevSystemUsec = systemUsec
		t.prevTotalUsec = totalUsec
		return 0.0, 0.0, 0.0
	}

	elapsed := now.Sub(t.prevTime).Microseconds()
	userPct := t.percent(t.prevUserUsec, userUsec, elapsed)
	systemPct := t.percent(t.prevSystemUsec, systemUsec, elapsed)
	totalPct := t.percent(t.prevTotalUsec, totalUsec, elapsed)
	t.prevUserUsec = userUsec
	t.prevSystemUsec = systemUsec
	t.prevTotalUsec = totalUsec
	t.prevTime = now
	return userPct, systemPct, totalPct
}

func (t *TrackCPU) percent(t1, t2 uint64, elapsed int64) float64 {
	delta := t2 - t1
	if elapsed <= 0 || delta <= 0.0 {
		return 0.0
	}
	return (float64(delta) / float64(elapsed)) * 100.0
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
