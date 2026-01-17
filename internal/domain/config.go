package domain

import (
	"time"
)

// Client represents an API client
type Client struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	APIKey    string    `json:"api_key"`
	RateLimit int       `json:"rate_limit"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
	LastUsed  time.Time `json:"last_used,omitempty"`
}

// InterpretationRecord represents a stored interpretation record
type InterpretationRecord struct {
	ID               string                 `json:"id"`
	VariantID        string                 `json:"variant_id"`
	ClientID         string                 `json:"client_id,omitempty"`
	RequestID        string                 `json:"request_id,omitempty"`
	Classification   Classification         `json:"classification"`
	ConfidenceLevel  ConfidenceLevel        `json:"confidence_level"`
	AppliedRules     []ACMGAMPRule          `json:"applied_rules"`
	EvidenceSummary  AggregatedEvidence     `json:"evidence_summary"`
	ReportData       map[string]interface{} `json:"report_data"` // Using map to avoid circular reference
	ProcessingTimeMs int                    `json:"processing_time_ms"`
	CreatedAt        time.Time              `json:"created_at"`
	UpdatedAt        time.Time              `json:"updated_at,omitempty"`
}

// Config represents the main application configuration
type Config struct {
	Server      ServerConfig      `mapstructure:"server"`
	Database    DatabaseConfig    `mapstructure:"database"`
	ExternalAPI ExternalAPIConfig `mapstructure:"external_api"`
	Cache       CacheConfig       `mapstructure:"cache"`
	Logging     LoggingConfig     `mapstructure:"logging"`
	MCP         MCPConfig         `mapstructure:"mcp"`
}

// ServerConfig represents HTTP server configuration
type ServerConfig struct {
	Host         string        `mapstructure:"host"`
	Port         int           `mapstructure:"port"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
	IdleTimeout  time.Duration `mapstructure:"idle_timeout"`
	TLSEnabled   bool          `mapstructure:"tls_enabled"`
	CertFile     string        `mapstructure:"cert_file"`
	KeyFile      string        `mapstructure:"key_file"`
}

// DatabaseConfig represents database connection configuration
type DatabaseConfig struct {
	Host            string        `mapstructure:"host"`
	Port            int           `mapstructure:"port"`
	Database        string        `mapstructure:"database"`
	Username        string        `mapstructure:"username"`
	Password        string        `mapstructure:"password"`
	SSLMode         string        `mapstructure:"ssl_mode"`
	MaxOpenConns    int           `mapstructure:"max_open_conns"`
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
}

// ExternalAPIConfig represents external API configuration
type ExternalAPIConfig struct {
	ClinVar ClinVarConfig `mapstructure:"clinvar"`
	GnomAD  GnomADConfig  `mapstructure:"gnomad"`
	COSMIC  COSMICConfig  `mapstructure:"cosmic"`
	PubMed  PubMedConfig  `mapstructure:"pubmed"`
	LOVD    LOVDConfig    `mapstructure:"lovd"`
	HGMD    HGMDConfig    `mapstructure:"hgmd"`
}

// ClinVarConfig represents ClinVar API configuration
type ClinVarConfig struct {
	BaseURL    string        `mapstructure:"base_url"`
	APIKey     string        `mapstructure:"api_key"`
	Timeout    time.Duration `mapstructure:"timeout"`
	RateLimit  int           `mapstructure:"rate_limit"`
	RetryCount int           `mapstructure:"retry_count"`
}

// GnomADConfig represents gnomAD API configuration
type GnomADConfig struct {
	BaseURL    string        `mapstructure:"base_url"`
	APIKey     string        `mapstructure:"api_key"`
	Timeout    time.Duration `mapstructure:"timeout"`
	RateLimit  int           `mapstructure:"rate_limit"`
	RetryCount int           `mapstructure:"retry_count"`
}

// COSMICConfig represents COSMIC API configuration
type COSMICConfig struct {
	BaseURL    string        `mapstructure:"base_url"`
	APIKey     string        `mapstructure:"api_key"`
	Timeout    time.Duration `mapstructure:"timeout"`
	RateLimit  int           `mapstructure:"rate_limit"`
	RetryCount int           `mapstructure:"retry_count"`
}

// CacheConfig represents cache configuration
type CacheConfig struct {
	RedisURL    string        `mapstructure:"redis_url"`
	DefaultTTL  time.Duration `mapstructure:"default_ttl"`
	MaxRetries  int           `mapstructure:"max_retries"`
	PoolSize    int           `mapstructure:"pool_size"`
	PoolTimeout time.Duration `mapstructure:"pool_timeout"`
}

// LoggingConfig represents logging configuration
type LoggingConfig struct {
	Level      string `mapstructure:"level"`
	Format     string `mapstructure:"format"`
	Output     string `mapstructure:"output"`
	Filename   string `mapstructure:"filename"`
	MaxSize    int    `mapstructure:"max_size"`
	MaxBackups int    `mapstructure:"max_backups"`
	MaxAge     int    `mapstructure:"max_age"`
	Compress   bool   `mapstructure:"compress"`
}

// MCPConfig represents MCP server configuration
type MCPConfig struct {
	ServerName       string        `mapstructure:"server_name"`
	ServerVersion    string        `mapstructure:"server_version"`
	TransportType    string        `mapstructure:"transport_type"`    // "stdio", "http"
	HTTPPort         int           `mapstructure:"http_port"`
	HTTPHost         string        `mapstructure:"http_host"`
	MaxClients       int           `mapstructure:"max_clients"`
	RequestTimeout   time.Duration `mapstructure:"request_timeout"`
	EnableMetrics    bool          `mapstructure:"enable_metrics"`
	EnableCaching    bool          `mapstructure:"enable_caching"`
	ToolCacheTTL     time.Duration `mapstructure:"tool_cache_ttl"`
	ResourceCacheTTL time.Duration `mapstructure:"resource_cache_ttl"`
}

// PubMedConfig represents PubMed API configuration
type PubMedConfig struct {
	BaseURL    string        `mapstructure:"base_url"`
	APIKey     string        `mapstructure:"api_key"`
	Email      string        `mapstructure:"email"`      // Required by NCBI
	Timeout    time.Duration `mapstructure:"timeout"`
	RateLimit  int           `mapstructure:"rate_limit"`
	RetryCount int           `mapstructure:"retry_count"`
}

// LOVDConfig represents LOVD API configuration
type LOVDConfig struct {
	BaseURL    string        `mapstructure:"base_url"`
	APIKey     string        `mapstructure:"api_key"`
	Timeout    time.Duration `mapstructure:"timeout"`
	RateLimit  int           `mapstructure:"rate_limit"`
	RetryCount int           `mapstructure:"retry_count"`
}

// HGMDConfig represents HGMD API configuration
type HGMDConfig struct {
	BaseURL        string        `mapstructure:"base_url"`
	APIKey         string        `mapstructure:"api_key"`
	License        string        `mapstructure:"license"`          // Professional license
	IsProfessional bool          `mapstructure:"is_professional"`  // Use professional API
	Timeout        time.Duration `mapstructure:"timeout"`
	RateLimit      int           `mapstructure:"rate_limit"`
	RetryCount     int           `mapstructure:"retry_count"`
}
