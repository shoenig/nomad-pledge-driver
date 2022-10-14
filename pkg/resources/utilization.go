package resources

type MicroSecond uint64

type Percent float64

type Utilization struct {
	Memory uint64
	Swap   uint64
	Cache  uint64

	System          Percent
	User            Percent
	Percent         Percent
	ThrottlePeriods uint64
	ThrottleTime    uint64
	Ticks           Percent
}
