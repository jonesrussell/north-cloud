package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server ServerConfig `yaml:"server"`
	Opers  []OperConfig `yaml:"opers"`
}

type ServerConfig struct {
	Name         string        `yaml:"name"`
	Network      string        `yaml:"network"`
	Listen       string        `yaml:"listen"`
	MaxClients   int           `yaml:"max_clients"`
	PingInterval time.Duration `yaml:"ping_interval"`
	PongTimeout  time.Duration `yaml:"pong_timeout"`
	MOTD         string        `yaml:"motd"`
}

type OperConfig struct {
	Name     string `yaml:"name"`
	Password string `yaml:"password"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	setDefaults(cfg)

	if err := validate(cfg); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}

	return cfg, nil
}

func setDefaults(cfg *Config) {
	if cfg.Server.Listen == "" {
		cfg.Server.Listen = "127.0.0.1:6667"
	}
	if cfg.Server.Network == "" {
		cfg.Server.Network = "NorthCloud"
	}
	if cfg.Server.MaxClients == 0 {
		cfg.Server.MaxClients = 256
	}
	if cfg.Server.PingInterval == 0 {
		cfg.Server.PingInterval = 90 * time.Second
	}
	if cfg.Server.PongTimeout == 0 {
		cfg.Server.PongTimeout = 120 * time.Second
	}
}

func validate(cfg *Config) error {
	if cfg.Server.Name == "" {
		return fmt.Errorf("server.name is required")
	}
	return nil
}
