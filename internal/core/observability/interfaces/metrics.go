package interfaces

type MetricsCollector interface {
	Counter(name string, tags map[string]string) Counter
	Gauge(name string, tags map[string]string) Gauge
	Histogram(name string, tags map[string]string) Histogram
	Timer(name string, tags map[string]string) Timer

	RegisterCallback(name string, callback MetricsCallback)
	Export() ([]MetricsFamily, error)
}

type (
	MetricsCallback func(name string, value float64, tags map[string]string)
	MetricsFamily   struct {
		Name    string
		Tags    map[string]string
		Metrics []any
	}
)

type Counter interface {
	Inc()
	Add(float64)
}

type Gauge interface {
	Set(float64)
	Inc()
	Dec()
	Add(float64)
	Sub(float64)
}

type Timer interface {
	Start()
	Stop()
	Reset()
}

type Histogram interface {
	Start()
	Stop()
	Reset()
	Count()
	Sum()
	Mean() float64
	Min() float64
	Max() float64
}
