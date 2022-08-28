package resources

type Utilization struct {
	Memory uint64
	Swap   uint64
	Cache  uint64

	System          float64
	User            float64
	Percent         float64
	ThrottlePeriods uint64
	ThrottleTime    uint64
	Ticks           float64
}
