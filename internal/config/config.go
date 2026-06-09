package config

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

type BackendConfig struct {
	URL string `json:"url"`
}

type HealthCheckConfig struct {
	Path               string        `json:"path"`
	Interval           time.Duration `json:"-"`
	Timeout            time.Duration `json:"-"`
	HealthyThreshold   int           `json:"healthy_threshold"`
	UnhealthyThreshold int           `json:"unhealthy_threshold"`

	IntervalRaw string `json:"interval"`
	TimeoutRaw  string `json:"timeout"`
}

type Config struct {
	Listen      string            `json:"listen"`
	Algorithm   string            `json:"algorithm"`
	HealthCheck HealthCheckConfig `json:"health_check"`
	Backends    []BackendConfig   `json:"backends"`
}

func Load(path string) (Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return Config{}, err
	}
	defer file.Close()

	var cfg Config
	if err := json.NewDecoder(file).Decode(&cfg); err != nil {
		return Config{}, err
	}

	if err := cfg.normalize(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func (cfg *Config) normalize() error {
	if cfg.Listen == "" {
		cfg.Listen = ":8080"
	}
	if cfg.Algorithm == "" {
		cfg.Algorithm = "round_robin"
	}
	if len(cfg.Backends) == 0 {
		return fmt.Errorf("at least one backend is required")
	}

	if cfg.HealthCheck.Path == "" {
		cfg.HealthCheck.Path = "/health"
	}
	if cfg.HealthCheck.IntervalRaw == "" {
		cfg.HealthCheck.IntervalRaw = "5s"
	}
	if cfg.HealthCheck.TimeoutRaw == "" {
		cfg.HealthCheck.TimeoutRaw = "2s"
	}

	interval, err := time.ParseDuration(cfg.HealthCheck.IntervalRaw)
	if err != nil {
		return fmt.Errorf("invalid health_check.interval: %w", err)
	}
	timeout, err := time.ParseDuration(cfg.HealthCheck.TimeoutRaw)
	if err != nil {
		return fmt.Errorf("invalid health_check.timeout: %w", err)
	}
	if interval <= 0 {
		return fmt.Errorf("health_check.interval must be positive")
	}
	if timeout <= 0 {
		return fmt.Errorf("health_check.timeout must be positive")
	}

	cfg.HealthCheck.Interval = interval
	cfg.HealthCheck.Timeout = timeout

	if cfg.HealthCheck.HealthyThreshold <= 0 {
		cfg.HealthCheck.HealthyThreshold = 1
	}
	if cfg.HealthCheck.UnhealthyThreshold <= 0 {
		cfg.HealthCheck.UnhealthyThreshold = 2
	}

	for i, backend := range cfg.Backends {
		if backend.URL == "" {
			return fmt.Errorf("backend[%d].url is required", i)
		}
	}

	return nil
}
