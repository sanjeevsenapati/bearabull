package config

import (
	"encoding/json"
	"os"
	"strconv"
	"time"
)

// Config represents the application configuration.
type Config struct {
	AppPort                   string        `json:"app_port"`
	AppEnv                    string        `json:"app_env"`
	DatabasePath              string        `json:"database_path"`
	YahooFinanceEnabled       bool          `json:"yahoo_finance_enabled"`
	MarketDataRefreshInterval time.Duration `json:"market_data_refresh_interval"`
	MaxConcurrentRequests     int           `json:"max_concurrent_requests"`
	RequestTimeout            time.Duration `json:"request_timeout"`
	CacheTTL                  time.Duration `json:"cache_ttl"`
	AnalysisTimezone          string        `json:"analysis_timezone"`
	LogLevel                  string        `json:"log_level"`
	AdminAuthEnabled          bool          `json:"admin_auth_enabled"`
	AdminAPIKey               string        `json:"admin_api_key"`
	StrategyVersion           string        `json:"strategy_version"`
}

// DefaultConfig returns default configuration parameters.
func DefaultConfig() *Config {
	return &Config{
		AppPort:                   "8080",
		AppEnv:                    "development",
		DatabasePath:              "./bearabull.db",
		YahooFinanceEnabled:       true,
		MarketDataRefreshInterval: 6 * time.Hour,
		MaxConcurrentRequests:     5,
		RequestTimeout:            15 * time.Second,
		CacheTTL:                  15 * time.Minute,
		AnalysisTimezone:          "Asia/Kolkata",
		LogLevel:                  "info",
		AdminAuthEnabled:          false,
		AdminAPIKey:               "bearabull-secret-key",
		StrategyVersion:           "v1.0.0",
	}
}

// Load loads configuration from an optional JSON file and overrides with environment variables.
func Load(filePath string) (*Config, error) {
	cfg := DefaultConfig()

	if filePath != "" {
		if data, err := os.ReadFile(filePath); err == nil {
			_ = json.Unmarshal(data, cfg)
		}
	}

	if port := os.Getenv("APP_PORT"); port != "" {
		cfg.AppPort = port
	}
	if env := os.Getenv("APP_ENV"); env != "" {
		cfg.AppEnv = env
	}
	if dbPath := os.Getenv("DATABASE_PATH"); dbPath != "" {
		cfg.DatabasePath = dbPath
	}
	if yf := os.Getenv("YAHOO_FINANCE_ENABLED"); yf != "" {
		cfg.YahooFinanceEnabled = yf == "true" || yf == "1"
	}
	if maxReq := os.Getenv("MAX_CONCURRENT_REQUESTS"); maxReq != "" {
		if v, err := strconv.Atoi(maxReq); err == nil && v > 0 {
			cfg.MaxConcurrentRequests = v
		}
	}
	if reqTimeout := os.Getenv("REQUEST_TIMEOUT"); reqTimeout != "" {
		if d, err := time.ParseDuration(reqTimeout); err == nil {
			cfg.RequestTimeout = d
		}
	}
	if tz := os.Getenv("ANALYSIS_TIMEZONE"); tz != "" {
		cfg.AnalysisTimezone = tz
	}
	if logLevel := os.Getenv("LOG_LEVEL"); logLevel != "" {
		cfg.LogLevel = logLevel
	}
	if adminAuth := os.Getenv("ADMIN_AUTH_ENABLED"); adminAuth != "" {
		cfg.AdminAuthEnabled = adminAuth == "true" || adminAuth == "1"
	}
	if apiKey := os.Getenv("ADMIN_API_KEY"); apiKey != "" {
		cfg.AdminAPIKey = apiKey
	}

	return cfg, nil
}
