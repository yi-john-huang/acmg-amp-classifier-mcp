package database

import (
	"context"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestDatabaseConnection(t *testing.T) {
	ctx := context.Background()

	// Start PostgreSQL container
	pgContainer, err := postgres.Run(ctx,
		"postgres:15-alpine",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second)),
	)
	if err != nil {
		t.Fatalf("Failed to start PostgreSQL container: %v", err)
	}
	defer func() {
		if err := pgContainer.Terminate(ctx); err != nil {
			t.Logf("Failed to terminate PostgreSQL container: %v", err)
		}
	}()

	// Get connection details
	host, err := pgContainer.Host(ctx)
	if err != nil {
		t.Fatalf("Failed to get container host: %v", err)
	}

	port, err := pgContainer.MappedPort(ctx, "5432")
	if err != nil {
		t.Fatalf("Failed to get container port: %v", err)
	}

	// Test database connection
	config := Config{
		Host:        host,
		Port:        port.Int(),
		Database:    "testdb",
		Username:    "testuser",
		Password:    "testpass",
		MaxConns:    10,
		MinConns:    2,
		MaxConnLife: time.Hour,
		MaxConnIdle: time.Minute * 30,
		SSLMode:     "disable",
	}

	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel) // Reduce noise in tests

	db, err := NewConnection(ctx, config, logger)
	if err != nil {
		t.Fatalf("Failed to create database connection: %v", err)
	}
	defer db.Close()

	// Test health check
	if err := db.Health(ctx); err != nil {
		t.Fatalf("Database health check failed: %v", err)
	}

	// Test connection pool stats
	stats := db.Stats()
	if stats.TotalConns() == 0 {
		t.Error("Expected at least one connection in pool")
	}

	t.Logf("Connection pool stats: Total=%d, Idle=%d, Used=%d",
		stats.TotalConns(), stats.IdleConns(), stats.AcquiredConns())
}
