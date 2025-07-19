package database

import (
	"context"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/sirupsen/logrus"
)

// MigrationRunner handles database migrations
type MigrationRunner struct {
	migrate *migrate.Migrate
	log     *logrus.Logger
}

// NewMigrationRunner creates a new migration runner
func NewMigrationRunner(databaseURL, migrationsPath string, logger *logrus.Logger) (*MigrationRunner, error) {
	m, err := migrate.New(
		fmt.Sprintf("file://%s", migrationsPath),
		databaseURL,
	)
	if err != nil {
		return nil, fmt.Errorf("creating migration instance: %w", err)
	}

	return &MigrationRunner{
		migrate: m,
		log:     logger,
	}, nil
}

// Up runs all pending migrations
func (mr *MigrationRunner) Up(ctx context.Context) error {
	mr.log.Info("Running database migrations up")

	if err := mr.migrate.Up(); err != nil {
		if err == migrate.ErrNoChange {
			mr.log.Info("No pending migrations to run")
			return nil
		}
		return fmt.Errorf("running migrations up: %w", err)
	}

	version, dirty, err := mr.migrate.Version()
	if err != nil {
		mr.log.WithError(err).Warn("Could not get migration version after up")
	} else {
		mr.log.WithFields(logrus.Fields{
			"version": version,
			"dirty":   dirty,
		}).Info("Migrations completed successfully")
	}

	return nil
}

// Down rolls back one migration
func (mr *MigrationRunner) Down(ctx context.Context) error {
	mr.log.Info("Rolling back one migration")

	if err := mr.migrate.Steps(-1); err != nil {
		if err == migrate.ErrNoChange {
			mr.log.Info("No migrations to roll back")
			return nil
		}
		return fmt.Errorf("rolling back migration: %w", err)
	}

	version, dirty, err := mr.migrate.Version()
	if err != nil {
		mr.log.WithError(err).Warn("Could not get migration version after down")
	} else {
		mr.log.WithFields(logrus.Fields{
			"version": version,
			"dirty":   dirty,
		}).Info("Migration rolled back successfully")
	}

	return nil
}

// Version returns the current migration version
func (mr *MigrationRunner) Version() (uint, bool, error) {
	return mr.migrate.Version()
}

// Close closes the migration runner
func (mr *MigrationRunner) Close() error {
	sourceErr, dbErr := mr.migrate.Close()
	if sourceErr != nil {
		return fmt.Errorf("closing migration source: %w", sourceErr)
	}
	if dbErr != nil {
		return fmt.Errorf("closing migration database: %w", dbErr)
	}
	return nil
}
