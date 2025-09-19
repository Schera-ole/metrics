package migration

import (
	"context"
	"database/sql"
	"fmt"
	"runtime"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"go.uber.org/zap"
)

func RunMigrations(ctx context.Context, dsn string, logger *zap.SugaredLogger) error {
	logger.Info("Running database migrations...")

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("failed to open database connection: %w", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}

	_, filename, _, _ := runtime.Caller(0)
	logger.Debugf("Current file: %s", filename)

	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("failed to create postgres driver: %w", err)
	}

	m, err := migrate.NewWithDatabaseInstance(
		"file://./migrations",
		"postgres", driver)
	if err != nil {
		return fmt.Errorf("failed to create migrate instance: %w", err)
	}

	version, dirty, err := m.Version()
	if err != nil && err != migrate.ErrNilVersion {
		logger.Warnf("Failed to get current migration version: %v", err)
	} else {
		logger.Infof("Current migration version: %d, dirty: %t", version, dirty)
	}

	if err := m.Up(); err != nil {
		if err == migrate.ErrNoChange {
			logger.Info("No new migrations to apply")
		} else {
			return fmt.Errorf("failed to run migrations: %w", err)
		}
	} else {
		logger.Info("Migrations applied successfully")
	}

	return nil
}
