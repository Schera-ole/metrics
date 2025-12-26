package repository

import (
	"context"
	"database/sql"
	"fmt"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/Schera-ole/metrics/internal/config"
	internalerrors "github.com/Schera-ole/metrics/internal/errors"
	models "github.com/Schera-ole/metrics/internal/model"
)

// DBStorage implements the Repository interface using a PostgreSQL database.
type DBStorage struct {
	// db is the underlying database connection
	db *sql.DB
}

// NewDBStorage creates a new database storage instance.
func NewDBStorage(dsn string) (*DBStorage, error) {
	dbConnect, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}
	return &DBStorage{db: dbConnect}, nil
}

// Close releases any resources held by the database storage.
func (storage *DBStorage) Close() error {
	return storage.db.Close()
}

// checkMetricExists checks if a metric exists and is not soft deleted
func (storage *DBStorage) checkMetricExists(ctx context.Context, tx *sql.Tx, name string) (bool, error) {
	var exists bool
	query := "SELECT EXISTS(SELECT 1 FROM metrics WHERE name = $1 AND deleted_at IS NULL)"
	err := tx.QueryRowContext(ctx, query, name).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("error checking if metric exists: %w", err)
	}
	return exists, nil
}

// insertMetric inserts a new metric record
func (storage *DBStorage) insertMetric(tx *sql.Tx, name string, typ string, value any) error {
	query := "INSERT INTO metrics (name, type, value, created_at, updated_at, deleted_at) VALUES ($1, $2, $3, NOW(), NOW(), NULL)"
	_, err := tx.Exec(query, name, typ, value)
	if err != nil {
		return fmt.Errorf("error saving metric: %w", err)
	}
	return nil
}

// updateMetric updates an existing metric based on its type
func (storage *DBStorage) updateMetric(ctx context.Context, tx *sql.Tx, name string, value any, typ string) error {
	switch typ {
	case config.CounterType:
		// For counters, increment the existing value
		query := "UPDATE metrics SET value = value + $1, updated_at = NOW() where name = $2 AND deleted_at IS NULL"
		_, err := tx.ExecContext(ctx, query, value, name)
		if err != nil {
			return fmt.Errorf("error saving metric: %w", err)
		}
	case config.GaugeType:
		// For gauges, replace the existing value
		query := "UPDATE metrics SET value = $1, updated_at = NOW() where name = $2 AND deleted_at IS NULL"
		_, err := tx.ExecContext(ctx, query, value, name)
		if err != nil {
			return fmt.Errorf("error saving metric: %w", err)
		}
	default:
		return fmt.Errorf("unknown metric type: %s", typ)
	}
	return nil
}

// SetMetrics saves multiple metrics in a single transaction.
//
// For each metric, it checks if it exists and either creates a new record or updates the existing one.
func (storage *DBStorage) SetMetrics(ctx context.Context, metrics []models.Metric) error {
	// Start a transaction to ensure atomicity of batch operations
	tx, err := storage.db.Begin()
	if err != nil {
		return fmt.Errorf("can't starting transaction: %w", err)
	}
	// Prepare statement to check if a metric already exists (not soft deleted)
	stmtExist, err := tx.PrepareContext(ctx, "SELECT EXISTS(SELECT 1 FROM metrics WHERE name = $1 AND deleted_at IS NULL)")
	if err != nil {
		return fmt.Errorf("error checking if metric exists: %w", err)
	}
	defer stmtExist.Close()
	for _, metric := range metrics {
		var exists bool
		err = stmtExist.QueryRowContext(ctx, metric.Name).Scan(&exists)
		if err != nil {
			return fmt.Errorf("error checking if metric exists: %w", err)
		}
		if !exists {
			// Insert new metric record
			err = storage.insertMetric(tx, metric.Name, metric.Type, metric.Value)
			if err != nil {
				return err
			}
		} else {
			// Update existing metric based on its type
			err = storage.updateMetric(ctx, tx, metric.Name, metric.Value, metric.Type)
			if err != nil {
				return err
			}
		}
	}
	// Commit the transaction to persist all changes
	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("error committing transaction: %w", err)
	}
	return nil
}

// SetMetric saves a single metric.
//
// It checks if the metric exists and either creates a new record or updates the existing one.
func (storage *DBStorage) SetMetric(ctx context.Context, name string, value any, typ string) error {
	tx, err := storage.db.Begin()
	if err != nil {
		return fmt.Errorf("can't start transaction: %w", err)
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	exists, err := storage.checkMetricExists(ctx, tx, name)
	if err != nil {
		return err
	}

	if !exists {
		// Insert new metric record
		err = storage.insertMetric(tx, name, typ, value)
		if err != nil {
			return err
		}
	} else {
		// Update existing metric based on its type
		err = storage.updateMetric(ctx, tx, name, value, typ)
		if err != nil {
			return err
		}
	}

	// Commit the transaction to persist changes
	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("error committing transaction: %w", err)
	}

	return nil
}

// GetMetric retrieves a single metric by its DTO.
//
// It returns a MetricsDTO with the current value of the requested metric.
func (storage *DBStorage) GetMetric(ctx context.Context, metrics models.MetricsDTO) (models.MetricsDTO, error) {
	var metricType string
	var value float64

	query := "SELECT type, value FROM metrics WHERE name = $1 AND deleted_at IS NULL"
	err := storage.db.QueryRowContext(ctx, query, metrics.ID).Scan(&metricType, &value)
	if err != nil {
		if err == sql.ErrNoRows {
			return models.MetricsDTO{}, internalerrors.ErrMetricNotFound
		}
		return models.MetricsDTO{}, fmt.Errorf("error retrieving metric: %w", err)
	}

	responseMetrics := models.MetricsDTO{
		ID:    metrics.ID,
		MType: metrics.MType,
	}

	switch metricType {
	case config.GaugeType:
		responseMetrics.Value = &value
	case config.CounterType:
		intValue := int64(value)
		responseMetrics.Delta = &intValue
	default:
		return models.MetricsDTO{}, internalerrors.ErrUnknownMetricType
	}
	return responseMetrics, nil
}

// GetMetricByName retrieves a single metric by its name.
//
// It returns the raw value of the requested metric (float64 for gauges, int64 for counters).
func (storage *DBStorage) GetMetricByName(ctx context.Context, name string) (any, error) {
	var metricType string
	var value float64

	query := "SELECT type, value FROM metrics WHERE name = $1 AND deleted_at IS NULL"
	err := storage.db.QueryRowContext(ctx, query, name).Scan(&metricType, &value)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, internalerrors.ErrMetricNotFound
		}
		return nil, fmt.Errorf("error retrieving metric: %w", err)
	}
	switch metricType {
	case config.GaugeType:
		return value, nil
	case config.CounterType:
		intValue := int64(value)
		return intValue, nil
	default:
		return nil, internalerrors.ErrUnknownMetricType
	}
}

// DeleteMetric removes a metric by its name using soft deletion.
//
// It sets the deleted_at timestamp for the metric, marking it as deleted without actually removing it from the database.
func (storage *DBStorage) DeleteMetric(ctx context.Context, name string) error {
	// Soft delete: set deleted_at timestamp
	query := "UPDATE metrics SET deleted_at = NOW() WHERE name = $1 AND deleted_at IS NULL"
	_, err := storage.db.ExecContext(ctx, query, name)
	if err != nil {
		return fmt.Errorf("error soft deleting metric: %w", err)
	}
	return nil
}

// ListMetrics retrieves all metrics that are not soft deleted.
//
// It returns a slice of Metric structs containing all gauge and counter values.
func (storage *DBStorage) ListMetrics(ctx context.Context) ([]models.Metric, error) {
	var formattedMetrics []models.Metric
	query := "SELECT name, type, value FROM metrics WHERE deleted_at IS NULL"
	rows, err := storage.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("error retrieving metrics: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var name, metricType string
		var value float64

		err = rows.Scan(&name, &metricType, &value)
		if err != nil {
			return nil, fmt.Errorf("error scanning metric: %w", err)
		}

		var metricValue any
		if metricType == config.CounterType {
			metricValue = int64(value)
		} else {
			metricValue = value
		}
		metric := models.Metric{
			Name:  name,
			Type:  metricType,
			Value: metricValue,
		}

		formattedMetrics = append(formattedMetrics, metric)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over metrics: %w", err)
	}

	return formattedMetrics, nil
}

// Ping checks the database connection health.
//
// It uses the database's built-in ping functionality to verify connectivity.
func (storage *DBStorage) Ping(ctx context.Context) error {
	err := storage.db.PingContext(ctx)
	if err != nil {
		return fmt.Errorf("%w: %v", internalerrors.ErrDatabaseConnection, err)
	}
	return nil
}
