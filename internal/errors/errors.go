package errors

import "errors"

var (
	// Common errors
	ErrMetricNotFound     = errors.New("metric not found")
	ErrUnknownMetricType  = errors.New("unknown metric type")
	ErrInvalidMetricValue = errors.New("invalid metric value")

	// Database errors
	ErrDatabaseConnection = errors.New("database connection failed")
	ErrTransactionFailed  = errors.New("transaction failed")
	ErrQueryExecution     = errors.New("query execution failed")

	// Storage errors
	ErrStorageUnavailable = errors.New("storage unavailable")
)
