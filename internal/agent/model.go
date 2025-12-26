// Package agent provides data structures and constants for the metrics collection agent.
package agent

// Metric represents a single collected metric with its name, type, and value.
type Metric struct {
	// Name is the unique identifier for the metric
	Name string

	// Type is the type of the metric (either "counter" or "gauge")
	Type string

	// Value is the metric value (int64 for counters, float64 for gauges)
	Value any
}

var (
	// RuntimeMetrics is a list of Go runtime metrics to collect.
	//
	// These metrics provide information about memory usage, garbage collection,
	// and other runtime statistics.
	RuntimeMetrics = []string{
		"Alloc",
		"BuckHashSys",
		"Frees",
		"GCCPUFraction",
		"GCSys",
		"HeapAlloc",
		"HeapIdle",
		"HeapInuse",
		"HeapObjects",
		"HeapReleased",
		"HeapSys",
		"LastGC",
		"Lookups",
		"MCacheInuse",
		"MCacheSys",
		"MSpanInuse",
		"MSpanSys",
		"Mallocs",
		"NextGC",
		"NumForcedGC",
		"NumGC",
		"OtherSys",
		"PauseTotalNs",
		"StackInuse",
		"StackSys",
		"Sys",
		"TotalAlloc",
	}
)
