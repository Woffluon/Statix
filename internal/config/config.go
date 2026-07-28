package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	ListenAddr             string `yaml:"listen_addr"`
	Domain                 string `yaml:"domain"`
	TLSEnabled             bool   `yaml:"tls_enabled"`
	AdminUsername          string `yaml:"admin_username"`
	AdminPasswordHash      string `yaml:"admin_password_hash"`
	SessionSecret          string `yaml:"session_secret"`
	SetupComplete          bool   `yaml:"setup_complete"`
	CollectIntervalSeconds int    `yaml:"collect_interval_seconds"`
	HistoryDurationHours   int    `yaml:"history_duration_hours"`
	LogFormat              string `yaml:"log_format"`
}

// DefaultConfig returns sensible default settings.
func DefaultConfig() *Config {
	return &Config{
		ListenAddr:             ":8080",
		SetupComplete:          false,
		CollectIntervalSeconds: 2,
		HistoryDurationHours:   6,
		LogFormat:              "json",
	}
}

// Load reads and validates configuration from path.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("config: failed to read file %s: %w", path, err)
	}

	cfg := DefaultConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("config: failed to unmarshal YAML from %s: %w", path, err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config: validation failed for %s: %w", path, err)
	}

	return cfg, nil
}

// Validate checks all configuration fields for correctness.
func (c *Config) Validate() error {
	if strings.TrimSpace(c.ListenAddr) == "" {
		return fmt.Errorf("listen_addr must not be empty")
	}
	if c.CollectIntervalSeconds < 1 {
		return fmt.Errorf("collect_interval_seconds must be >= 1, got %d", c.CollectIntervalSeconds)
	}
	if c.HistoryDurationHours < 1 {
		return fmt.Errorf("history_duration_hours must be >= 1, got %d", c.HistoryDurationHours)
	}
	switch c.LogFormat {
	case "json", "text":
		// valid
	default:
		return fmt.Errorf("log_format must be 'json' or 'text', got %q", c.LogFormat)
	}
	return nil
}

// Save writes config atomically to path with 0600 permissions.
func Save(path string, cfg *Config) error {
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("config: save validation failed: %w", err)
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("config: failed to create directory %s: %w", dir, err)
	}

	tmp, err := os.CreateTemp(dir, ".statix-config-*")
	if err != nil {
		return fmt.Errorf("config: failed to create temp file: %w", err)
	}
	tmpName := tmp.Name()
	defer func() {
		_ = tmp.Close()
		_ = os.Remove(tmpName) // no-op if already renamed
	}()

	encoder := yaml.NewEncoder(tmp)
	encoder.SetIndent(2)
	if err := encoder.Encode(cfg); err != nil {
		return fmt.Errorf("config: failed to encode YAML: %w", err)
	}

	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("config: failed to sync temp file: %w", err)
	}

	if err := tmp.Close(); err != nil {
		return fmt.Errorf("config: failed to close temp file: %w", err)
	}

	if err := os.Chmod(tmpName, 0600); err != nil {
		return fmt.Errorf("config: failed to chmod temp file: %w", err)
	}

	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("config: failed to rename temp file to %s: %w", path, err)
	}

	return nil
}
