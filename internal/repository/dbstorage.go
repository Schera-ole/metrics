package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/Schera-ole/metrics/internal/config"
	models "github.com/Schera-ole/metrics/internal/model"
	_ "github.com/jackc/pgx/v5/stdlib"
)

type DBStorage struct {
	db *sql.DB
}

func NewDBStorage(dsn string) (*DBStorage, error) {
	dbConnect, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}
	return &DBStorage{db: dbConnect}, nil
}

func (storage *DBStorage) Close() error {
	return storage.db.Close()
}

func (storage *DBStorage) SetMetrics(ctx context.Context, metrics []models.Metric) error {
	tx, err := storage.db.Begin()
	if err != nil {
		return fmt.Errorf("can't starting transaction: %w", err)
	}
	stmtExist, err := tx.PrepareContext(ctx, "SELECT EXISTS(SELECT 1 FROM metrics WHERE name = $1)")
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
			query := "INSERT INTO metrics (name, type, value, created_at, updated_at) VALUES ($1, $2, $3, NOW(), NOW())"
			_, err = tx.Exec(query, metric.Name, metric.Type, metric.Value)
			if err != nil {
				return fmt.Errorf("error saving metric: %w", err)
			}
		} else {
			switch metric.Type {
			case config.CounterType:
				query := "UPDATE metrics SET value = value + $1, updated_at = NOW() where name = $2"
				_, err := tx.ExecContext(ctx, query, metric.Value, metric.Name)
				if err != nil {
					return fmt.Errorf("error saving metric: %w", err)
				}
			case config.GaugeType:
				query := "UPDATE metrics SET value = $1, updated_at = NOW() where name = $2"
				_, err := tx.ExecContext(ctx, query, metric.Value, metric.Name)
				if err != nil {
					return fmt.Errorf("error saving metric: %w", err)
				}
			}
		}
	}
	tx.Commit()
	return nil
}

func (storage *DBStorage) SetMetric(ctx context.Context, name string, value any, typ string) error {
	var exists bool
	query := "SELECT EXISTS(SELECT 1 FROM metrics WHERE name = $1)"
	err := storage.db.QueryRowContext(ctx, query, name).Scan(&exists)
	if err != nil {
		return fmt.Errorf("error checking if metric exists: %w", err)
	}
	if !exists {
		query = "INSERT INTO metrics (name, type, value, created_at, updated_at) VALUES ($1, $2, $3, NOW(), NOW())"
		_, err = storage.db.Exec(query, name, typ, value)
		if err != nil {
			return fmt.Errorf("error saving metric: %w", err)
		}
	} else {
		switch typ {
		case config.CounterType:
			query = "UPDATE metrics SET value = value + $1, updated_at = NOW() where name = $2"
			_, err := storage.db.ExecContext(ctx, query, value, name)
			if err != nil {
				return fmt.Errorf("error saving metric: %w", err)
			}
		case config.GaugeType:
			query = "UPDATE metrics SET value = $1, updated_at = NOW() where name = $2"
			_, err := storage.db.ExecContext(ctx, query, value, name)
			if err != nil {
				return fmt.Errorf("error saving metric: %w", err)
			}
		}
	}
	return nil
}

func (storage *DBStorage) GetMetric(ctx context.Context, metrics models.MetricsDTO) (models.MetricsDTO, error) {
	var metricType string
	var value float64

	query := "SELECT type, value FROM metrics WHERE name = $1"
	err := storage.db.QueryRowContext(ctx, query, metrics.ID).Scan(&metricType, &value)
	if err != nil {
		if err == sql.ErrNoRows {
			return models.MetricsDTO{}, fmt.Errorf("metric not found")
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
		return models.MetricsDTO{}, fmt.Errorf("unknown type of metric")
	}
	return responseMetrics, nil
}

func (storage *DBStorage) GetMetricByName(ctx context.Context, name string) (any, error) {
	var metricType string
	var value float64

	query := "SELECT type, value FROM metrics WHERE name = $1"
	err := storage.db.QueryRowContext(ctx, query, name).Scan(&metricType, &value)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("metric not found")
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
		return nil, fmt.Errorf("unknown type of metric")
	}
}

func (storage *DBStorage) DeleteMetric(ctx context.Context, name string) error {
	query := "DELETE FROM metrics WHERE name = $1"
	_, err := storage.db.Exec(query, name)
	if err != nil {
		return fmt.Errorf("error deleting metric: %w", err)
	}
	return nil
}

func (storage *DBStorage) ListMetrics(ctx context.Context) ([]models.Metric, error) {
	var formattedMetrics []models.Metric
	query := "SELECT name, type, value from metrics"
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

func (storage *DBStorage) Ping(ctx context.Context) error {
	err := storage.db.PingContext(ctx)
	if err != nil {
		return fmt.Errorf("database ping failed: %v", err)
	}
	return nil
}
