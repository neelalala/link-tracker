package database

import (
	"errors"
	"fmt"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"log/slog"
)

func RunMigrationsFromFile(databaseUrl, migrationsPath string, logger *slog.Logger) error {
	logger.Info("running database migrations")

	m, err := migrate.New(migrationsPath, databaseUrl)
	if err != nil {
		return fmt.Errorf("failed to init migrate instance: %w", err)
	}
	defer m.Close()

	err = m.Up()
	if err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			logger.Info("database is up to date, no migrations applied")
			return nil
		}
		return fmt.Errorf("failed to apply migrations: %w", err)
	}

	logger.Info("database migrations applied successfully")
	return nil
}
