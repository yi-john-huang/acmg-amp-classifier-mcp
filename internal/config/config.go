package config

import (
	"fmt"
	"strings"

	"github.com/acmg-amp-mcp-server/internal/domain"
	"github.com/spf13/viper"
)

// Manager implements the ConfigManager interface using Viper
type Manager struct {
	config *domain.Config
}

// NewManager creates a new configuration manager
func NewManager() (*Manager, error) {
	m := &Manager{}
	if err := m.loadConfig(); err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}
	return m, nil
}

// loadConfig loads configuration from various sources
func (m *Manager) loadConfig() error {
	// Set configuration file name and paths
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("./config")
	viper.AddConfigPath("/etc/acmg-amp-mcp-server/")

	// Set environment variable prefix and enable automatic env binding
	viper.SetEnvPrefix("ACMG_AMP")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	// Set default values
	m.setDefaults()

	// Read configuration file (optional - will use defaults and env vars if not found)
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return fmt.Errorf("error reading config file: %w", err)
		}
		// Config file not found; using defaults and environment variables
	}

	// Unmarshal configuration into struct
	config := &domain.Config{}
	if err := viper.Unmarshal(config); err != nil {
		return fmt.Errorf("error unmarshaling config: %w", err)
	}

	m.config = config
	return nil
}

// setDefaults sets default configuration values
func (m *Manager) setDefaults() {
	// Server defaults
	viper.SetDefault("server.host", "0.0.0.0")
	viper.SetDefault("server.port", 8080)
	viper.SetDefault("server.read_timeout", "30s")
	viper.SetDefault("server.write_timeout", "30s")
	viper.SetDefault("server.idle_timeout", "120s")
	viper.SetDefault("server.tls_enabled", false)

	// Database defaults
	viper.SetDefault("database.host", "localhost")
	viper.SetDefault("database.port", 5432)
	viper.SetDefault("database.database", "acmg_amp_mcp")
	viper.SetDefault("database.username", "postgres")
	viper.SetDefault("database.password", "")
	viper.SetDefault("database.ssl_mode", "disable")
	viper.SetDefault("database.max_open_conns", 25)
	viper.SetDefault("database.max_idle_conns", 5)
	viper.SetDefault("database.conn_max_lifetime", "5m")

	// External API defaults
	viper.SetDefault("external_api.clinvar.base_url", "https://eutils.ncbi.nlm.nih.gov/entrez/eutils/")
	viper.SetDefault("external_api.clinvar.timeout", "30s")
	viper.SetDefault("external_api.clinvar.rate_limit", 10)
	viper.SetDefault("external_api.clinvar.retry_count", 3)

	viper.SetDefault("external_api.gnomad.base_url", "https://gnomad.broadinstitute.org/api/")
	viper.SetDefault("external_api.gnomad.timeout", "30s")
	viper.SetDefault("external_api.gnomad.rate_limit", 10)
	viper.SetDefault("external_api.gnomad.retry_count", 3)

	viper.SetDefault("external_api.cosmic.base_url", "https://cancer.sanger.ac.uk/cosmic/")
	viper.SetDefault("external_api.cosmic.timeout", "30s")
	viper.SetDefault("external_api.cosmic.rate_limit", 10)
	viper.SetDefault("external_api.cosmic.retry_count", 3)

	// Cache defaults
	viper.SetDefault("cache.redis_url", "redis://localhost:6379")
	viper.SetDefault("cache.default_ttl", "24h")
	viper.SetDefault("cache.max_retries", 3)
	viper.SetDefault("cache.pool_size", 10)
	viper.SetDefault("cache.pool_timeout", "4s")

	// Logging defaults
	viper.SetDefault("logging.level", "info")
	viper.SetDefault("logging.format", "json")
	viper.SetDefault("logging.output", "stdout")
	viper.SetDefault("logging.max_size", 100)
	viper.SetDefault("logging.max_backups", 3)
	viper.SetDefault("logging.max_age", 28)
	viper.SetDefault("logging.compress", true)
}

// GetConfig returns the complete configuration
func (m *Manager) GetConfig() *domain.Config {
	return m.config
}

// GetDatabaseConfig returns database configuration
func (m *Manager) GetDatabaseConfig() *domain.DatabaseConfig {
	return &m.config.Database
}

// GetExternalAPIConfig returns external API configuration
func (m *Manager) GetExternalAPIConfig() *domain.ExternalAPIConfig {
	return &m.config.ExternalAPI
}

// GetServerConfig returns server configuration
func (m *Manager) GetServerConfig() *domain.ServerConfig {
	return &m.config.Server
}

// Reload reloads the configuration
func (m *Manager) Reload() error {
	return m.loadConfig()
}

// Validate validates the configuration
func (m *Manager) Validate() error {
	config := m.config

	// Validate server configuration
	if config.Server.Port <= 0 || config.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", config.Server.Port)
	}

	// Validate database configuration
	if config.Database.Host == "" {
		return fmt.Errorf("database host is required")
	}
	if config.Database.Database == "" {
		return fmt.Errorf("database name is required")
	}
	if config.Database.Username == "" {
		return fmt.Errorf("database username is required")
	}

	// Validate external API URLs
	if config.ExternalAPI.ClinVar.BaseURL == "" {
		return fmt.Errorf("ClinVar base URL is required")
	}
	if config.ExternalAPI.GnomAD.BaseURL == "" {
		return fmt.Errorf("gnomAD base URL is required")
	}
	if config.ExternalAPI.COSMIC.BaseURL == "" {
		return fmt.Errorf("COSMIC base URL is required")
	}

	// Validate cache configuration
	if config.Cache.RedisURL == "" {
		return fmt.Errorf("Redis URL is required")
	}

	// Validate logging configuration
	validLogLevels := map[string]bool{
		"debug": true, "info": true, "warn": true, "error": true, "fatal": true, "panic": true,
	}
	if !validLogLevels[strings.ToLower(config.Logging.Level)] {
		return fmt.Errorf("invalid log level: %s", config.Logging.Level)
	}

	return nil
}

// GetDatabaseConnectionString returns a formatted database connection string
func (m *Manager) GetDatabaseConnectionString() string {
	db := m.config.Database
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		db.Host, db.Port, db.Username, db.Password, db.Database, db.SSLMode)
}

// GetRedisConnectionString returns the Redis connection string
func (m *Manager) GetRedisConnectionString() string {
	return m.config.Cache.RedisURL
}

// IsProduction returns true if running in production mode
func (m *Manager) IsProduction() bool {
	return strings.ToLower(viper.GetString("environment")) == "production"
}

// IsDevelopment returns true if running in development mode
func (m *Manager) IsDevelopment() bool {
	env := strings.ToLower(viper.GetString("environment"))
	return env == "development" || env == "dev" || env == ""
}
