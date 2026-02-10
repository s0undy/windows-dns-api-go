package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server     ServerConfig     `yaml:"server"`
	DNS        DNSConfig        `yaml:"dns"`
	PowerShell PowerShellConfig `yaml:"powershell"`
	Logging    LoggingConfig    `yaml:"logging"`
	APIKeys    []APIKey         `yaml:"api_keys"`
}

type ServerConfig struct {
	Address      string        `yaml:"address"`
	Port         int           `yaml:"port"`
	ReadTimeout  time.Duration `yaml:"read_timeout"`
	WriteTimeout time.Duration `yaml:"write_timeout"`
}

type DNSConfig struct {
	ServerName  string `yaml:"server_name"`
	DefaultZone string `yaml:"default_zone"`
}

type PowerShellConfig struct {
	Timeout    time.Duration `yaml:"timeout"`
	Executable string        `yaml:"executable"`
}

type LoggingConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
}

type APIKey struct {
	Name string `yaml:"name"`
	Key  string `yaml:"key"`
}

// Load reads and parses the configuration file
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Apply defaults
	applyDefaults(&cfg)

	// Validate
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &cfg, nil
}

// applyDefaults sets default values for optional fields
func applyDefaults(cfg *Config) {
	if cfg.Server.Address == "" {
		cfg.Server.Address = "0.0.0.0"
	}
	if cfg.Server.Port == 0 {
		cfg.Server.Port = 8080
	}
	if cfg.Server.ReadTimeout == 0 {
		cfg.Server.ReadTimeout = 10 * time.Second
	}
	if cfg.Server.WriteTimeout == 0 {
		cfg.Server.WriteTimeout = 10 * time.Second
	}

	if cfg.DNS.ServerName == "" {
		cfg.DNS.ServerName = "."
	}

	if cfg.PowerShell.Timeout == 0 {
		cfg.PowerShell.Timeout = 30 * time.Second
	}
	if cfg.PowerShell.Executable == "" {
		cfg.PowerShell.Executable = "powershell.exe"
	}

	if cfg.Logging.Level == "" {
		cfg.Logging.Level = "info"
	}
	if cfg.Logging.Format == "" {
		cfg.Logging.Format = "json"
	}
}

// Validate checks the configuration for errors
func (c *Config) Validate() error {
	if c.Server.Port < 1 || c.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", c.Server.Port)
	}

	if c.DNS.DefaultZone == "" {
		return fmt.Errorf("dns.default_zone is required")
	}

	validLogLevels := map[string]bool{
		"debug": true,
		"info":  true,
		"warn":  true,
		"error": true,
	}
	if !validLogLevels[c.Logging.Level] {
		return fmt.Errorf("invalid logging level: %s (must be debug, info, warn, or error)", c.Logging.Level)
	}

	validLogFormats := map[string]bool{
		"json": true,
		"text": true,
	}
	if !validLogFormats[c.Logging.Format] {
		return fmt.Errorf("invalid logging format: %s (must be json or text)", c.Logging.Format)
	}

	if len(c.APIKeys) == 0 {
		return fmt.Errorf("at least one API key is required")
	}

	for i, key := range c.APIKeys {
		if key.Name == "" {
			return fmt.Errorf("api_keys[%d]: name is required", i)
		}
		if key.Key == "" {
			return fmt.Errorf("api_keys[%d]: key is required", i)
		}
	}

	return nil
}

// GetAPIKeyMap returns a map of API keys for O(1) lookups
func (c *Config) GetAPIKeyMap() map[string]string {
	m := make(map[string]string, len(c.APIKeys))
	for _, key := range c.APIKeys {
		m[key.Key] = key.Name
	}
	return m
}
