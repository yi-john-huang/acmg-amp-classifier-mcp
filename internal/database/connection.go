package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirupsen/logrus"
)

// Config holds database configuration
type Config struct {
	Host        string
	Port        int
	Database    string
	Username    string
	Password    string
	MaxConns    int32
	MinConns    int32
	MaxConnLife time.Duration
	MaxConnIdle time.Duration
	SSLMode     string
}

// DB wraps the pgxpool.Pool with additional functionality
type DB struct {
	Pool *pgxpool.Pool
	log  *logrus.Logger
}

// NewConnection creates a new database connection pool
func NewConnection(ctx context.Context, config Config, logger *logrus.Logger) (*DB, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%d dbname=%s user=%s password=%s sslmode=%s",
		config.Host, config.Port, config.Database, config.Username, config.Password, config.SSLMode,
	)

	poolConfig, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parsing database config: %w", err)
	}

	// Configure connection pool settings
	poolConfig.MaxConns = config.MaxConns
	poolConfig.MinConns = config.MinConns
	poolConfig.MaxConnLifetime = config.MaxConnLife
	poolConfig.MaxConnIdleTime = config.MaxConnIdle

	// Create connection pool
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("creating connection pool: %w", err)
	}

	// Test the connection
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("pinging database: %w", err)
	}

	logger.WithFields(logrus.Fields{
		"host":      config.Host,
		"port":      config.Port,
		"database":  config.Database,
		"max_conns": config.MaxConns,
		"min_conns": config.MinConns,
	}).Info("Database connection pool established")

	return &DB{
		Pool: pool,
		log:  logger,
	}, nil
}

// Close closes the database connection pool
func (db *DB) Close() {
	if db.Pool != nil {
		db.Pool.Close()
		db.log.Info("Database connection pool closed")
	}
}

// Health checks the database connection health
func (db *DB) Health(ctx context.Context) error {
	return db.Pool.Ping(ctx)
}

// Stats returns connection pool statistics
func (db *DB) Stats() *pgxpool.Stat {
	return db.Pool.Stat()
}
