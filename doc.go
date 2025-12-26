// Package metrics implements a server for collecting and storing various system metrics.
//
// The server supports two types of metrics:
//   - Gauge: represents float64 values, typically used for measuring things
//   - Counter: represents int64 values, typically used for counting events or requests
//
// The server can store metrics in memory or in a PostgreSQL database. It also supports
// periodic saving of metrics to a file for persistence when using in-memory storage.
//
// Features:
//   - REST API for updating and retrieving metrics
//   - Support for batch updates
//   - Data compression using gzip
//   - Data integrity validation using HMAC SHA256 hashing
//   - Graceful shutdown handling
//   - Structured logging
//   - Profiling support via pprof
//   - Audit logging to file or HTTP endpoint
//
// The server includes an agent component that collects system metrics.
//
// Both server and agent components support configuration via command-line flags
// and environment variables.
package metrics
