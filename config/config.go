// config/config.go
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server    ServerConfig    `yaml:"server"`
	Store     StoreConfig     `yaml:"store"`
	Secrets   SecretsConfig   `yaml:"secrets"`
	RateLimit RateLimitConfig `yaml:"rate_limit"`
}

type ServerConfig struct {
	Host    string `yaml:"host"`
	Port    int    `yaml:"port"`
	BaseURL string `yaml:"base_url"`
}

type StoreConfig struct {
	Type  string      `yaml:"type"`
	Redis RedisConfig `yaml:"redis"`
}

type RedisConfig struct {
	Addr     string `yaml:"addr"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
}

type SecretsConfig struct {
	DefaultTTL   time.Duration `yaml:"default_ttl"`
	MaxTTL       time.Duration `yaml:"max_ttl"`
	DefaultViews int           `yaml:"default_views"`
	MaxViews     int           `yaml:"max_views"`
}

type RateLimitConfig struct {
	Enabled        bool `yaml:"enabled"`
	RequestsPerMin int  `yaml:"requests_per_min"`
	RevealPerMin   int  `yaml:"reveal_per_min"`
}

func Default() *Config {
	return &Config{
		Server: ServerConfig{
			Host:    "0.0.0.0",
			Port:    8080,
			BaseURL: "http://localhost:8080",
		},
		Store: StoreConfig{
			Type: "memory",
			Redis: RedisConfig{
				Addr:     "localhost:6379",
				Password: "",
				DB:       0,
			},
		},
		Secrets: SecretsConfig{
			DefaultTTL:   1 * time.Hour,
			MaxTTL:       24 * time.Hour,
			DefaultViews: 1,
			MaxViews:     10,
		},
		RateLimit: RateLimitConfig{
			Enabled:        true,
			RequestsPerMin: 100,
			RevealPerMin:   20,
		},
	}
}

func Load(path string) (*Config, error) {
	cfg := Default()

	if path != "" {
		if err := cfg.loadFromFile(path); err != nil {
			return nil, err
		}
	}

	cfg.loadFromEnv()

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) loadFromFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // File not found is OK, use defaults
		}
		return fmt.Errorf("reading config file: %w", err)
	}

	if err := yaml.Unmarshal(data, c); err != nil {
		return fmt.Errorf("parsing config file: %w", err)
	}

	return nil
}

func (c *Config) loadFromEnv() {
	// Server
	if v := os.Getenv("HOST"); v != "" {
		c.Server.Host = v
	}
	if v := os.Getenv("PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			c.Server.Port = port
		}
	}
	if v := os.Getenv("BASE_URL"); v != "" {
		c.Server.BaseURL = v
	}

	if v := os.Getenv("STORE_TYPE"); v != "" {
		c.Store.Type = v
	}
	if v := os.Getenv("REDIS_ADDR"); v != "" {
		c.Store.Redis.Addr = v
	}
	if v := os.Getenv("REDIS_PASSWORD"); v != "" {
		c.Store.Redis.Password = v
	}
	if v := os.Getenv("REDIS_DB"); v != "" {
		if db, err := strconv.Atoi(v); err == nil {
			c.Store.Redis.DB = db
		}
	}

	if v := os.Getenv("DEFAULT_TTL"); v != "" {
		if ttl, err := time.ParseDuration(v); err == nil {
			c.Secrets.DefaultTTL = ttl
		}
	}
	if v := os.Getenv("MAX_TTL"); v != "" {
		if ttl, err := time.ParseDuration(v); err == nil {
			c.Secrets.MaxTTL = ttl
		}
	}
	if v := os.Getenv("DEFAULT_VIEWS"); v != "" {
		if views, err := strconv.Atoi(v); err == nil {
			c.Secrets.DefaultViews = views
		}
	}
	if v := os.Getenv("MAX_VIEWS"); v != "" {
		if views, err := strconv.Atoi(v); err == nil {
			c.Secrets.MaxViews = views
		}
	}

	if v := os.Getenv("RATE_LIMIT_ENABLED"); v != "" {
		c.RateLimit.Enabled = v == "true" || v == "1"
	}
	if v := os.Getenv("RATE_LIMIT_REQUESTS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			c.RateLimit.RequestsPerMin = n
		}
	}
	if v := os.Getenv("RATE_LIMIT_REVEAL"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			c.RateLimit.RevealPerMin = n
		}
	}
}

func (c *Config) Validate() error {
	if c.Server.Port < 1 || c.Server.Port > 65535 {
		return fmt.Errorf("invalid port: %d", c.Server.Port)
	}

	if c.Server.BaseURL == "" {
		return fmt.Errorf("base_url is required")
	}

	if c.Store.Type != "memory" && c.Store.Type != "redis" {
		return fmt.Errorf("invalid store type: %s (must be 'memory' or 'redis')", c.Store.Type)
	}

	if c.Store.Type == "redis" && c.Store.Redis.Addr == "" {
		return fmt.Errorf("redis addr is required when store type is 'redis'")
	}

	if c.Secrets.DefaultTTL <= 0 {
		return fmt.Errorf("default_ttl must be positive")
	}

	if c.Secrets.MaxTTL < c.Secrets.DefaultTTL {
		return fmt.Errorf("max_ttl must be >= default_ttl")
	}

	if c.Secrets.DefaultViews < 1 {
		return fmt.Errorf("default_views must be at least 1")
	}

	if c.Secrets.MaxViews < c.Secrets.DefaultViews {
		return fmt.Errorf("max_views must be >= default_views")
	}

	return nil
}

func (c *Config) Addr() string {
	return fmt.Sprintf("%s:%d", c.Server.Host, c.Server.Port)
}
