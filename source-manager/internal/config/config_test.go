package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoad(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yml")
	configContent := `
debug: true
server:
  host: "0.0.0.0"
  port: 8050
database:
  host: "localhost"
  port: 5432
  user: "testuser"
  password: "testpass"
  dbname: "testdb"
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}

	if cfg == nil {
		t.Fatal("Load() returned nil config")
	}

	if !cfg.Debug {
		t.Error("Load() cfg.Debug = false, want true")
	}

	if cfg.Server.Host != "0.0.0.0" {
		t.Errorf("Load() cfg.Server.Host = %v, want 0.0.0.0", cfg.Server.Host)
	}

	if cfg.Server.Port != 8050 {
		t.Errorf("Load() cfg.Server.Port = %v, want 8050", cfg.Server.Port)
	}

	if cfg.Database.Host != "localhost" {
		t.Errorf("Load() cfg.Database.Host = %v, want localhost", cfg.Database.Host)
	}
}

func TestLoad_Defaults(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yml")
	configContent := `
server:
  host: "127.0.0.1"
database:
  host: "localhost"
  user: "user"
  password: "pass"
  dbname: "db"
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}

	// Check defaults
	if cfg.Server.Port != defaultServerPort {
		t.Errorf("Load() cfg.Server.Port = %v, want %v", cfg.Server.Port, defaultServerPort)
	}

	if cfg.Database.Port != defaultDatabasePort {
		t.Errorf("Load() cfg.Database.Port = %v, want %v", cfg.Database.Port, defaultDatabasePort)
	}

	if cfg.Database.SSLMode != "disable" {
		t.Errorf("Load() cfg.Database.SSLMode = %v, want disable", cfg.Database.SSLMode)
	}

	if cfg.Database.MaxOpenConns != defaultMaxOpenConns {
		t.Errorf("Load() cfg.Database.MaxOpenConns = %v, want %v", cfg.Database.MaxOpenConns, defaultMaxOpenConns)
	}

	if cfg.Server.ReadTimeout != defaultServerTimeout*time.Second {
		t.Errorf("Load() cfg.Server.ReadTimeout = %v, want %v", cfg.Server.ReadTimeout, defaultServerTimeout*time.Second)
	}
}

func TestLoad_FileNotFound(t *testing.T) {
	_, err := Load("/nonexistent/config.yml")
	if err == nil {
		t.Error("Load() error = nil, want error for nonexistent file")
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "invalid.yml")
	if err := os.WriteFile(configPath, []byte("invalid: yaml: content: ["), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	_, err := Load(configPath)
	if err == nil {
		t.Error("Load() error = nil, want error for invalid YAML")
	}
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: Config{
				Server: ServerConfig{
					Host: "0.0.0.0",
					Port: 8050,
				},
				Database: DatabaseConfig{
					Host:   "localhost",
					Port:   5432,
					User:   "user",
					DBName: "db",
				},
			},
			wantErr: false,
		},
		{
			name: "empty server host",
			config: Config{
				Server: ServerConfig{
					Port: 8050,
				},
				Database: DatabaseConfig{
					Host:   "localhost",
					Port:   5432,
					User:   "user",
					DBName: "db",
				},
			},
			wantErr: true,
		},
		{
			name: "zero server port",
			config: Config{
				Server: ServerConfig{
					Host: "0.0.0.0",
					Port: 0,
				},
				Database: DatabaseConfig{
					Host:   "localhost",
					Port:   5432,
					User:   "user",
					DBName: "db",
				},
			},
			wantErr: true,
		},
		{
			name: "empty database host",
			config: Config{
				Server: ServerConfig{
					Host: "0.0.0.0",
					Port: 8050,
				},
				Database: DatabaseConfig{
					Port:   5432,
					User:   "user",
					DBName: "db",
				},
			},
			wantErr: true,
		},
		{
			name: "empty database user",
			config: Config{
				Server: ServerConfig{
					Host: "0.0.0.0",
					Port: 8050,
				},
				Database: DatabaseConfig{
					Host:   "localhost",
					Port:   5432,
					DBName: "db",
				},
			},
			wantErr: true,
		},
		{
			name: "empty database name",
			config: Config{
				Server: ServerConfig{
					Host: "0.0.0.0",
					Port: 8050,
				},
				Database: DatabaseConfig{
					Host: "localhost",
					Port: 5432,
					User: "user",
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Config.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestOverrideFromEnv(t *testing.T) {
	// Save original env values
	originalDBHost := os.Getenv("DB_HOST")
	originalDBPort := os.Getenv("DB_PORT")
	originalDBUser := os.Getenv("DB_USER")
	originalServerHost := os.Getenv("SERVER_HOST")
	originalServerPort := os.Getenv("SERVER_PORT")
	originalAppDebug := os.Getenv("APP_DEBUG")

	// Clean up after test
	defer func() {
		os.Setenv("DB_HOST", originalDBHost)
		os.Setenv("DB_PORT", originalDBPort)
		os.Setenv("DB_USER", originalDBUser)
		os.Setenv("SERVER_HOST", originalServerHost)
		os.Setenv("SERVER_PORT", originalServerPort)
		os.Setenv("APP_DEBUG", originalAppDebug)
	}()

	// Set test environment variables
	os.Setenv("DB_HOST", "env-host")
	os.Setenv("DB_PORT", "5433")
	os.Setenv("DB_USER", "env-user")
	os.Setenv("SERVER_HOST", "env-server")
	os.Setenv("SERVER_PORT", "9000")
	os.Setenv("APP_DEBUG", "true")

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yml")
	configContent := `
server:
  host: "127.0.0.1"
  port: 8050
database:
  host: "localhost"
  port: 5432
  user: "user"
  password: "pass"
  dbname: "db"
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}

	// Check that environment variables override config
	if cfg.Database.Host != "env-host" {
		t.Errorf("Load() cfg.Database.Host = %v, want env-host", cfg.Database.Host)
	}

	if cfg.Database.Port != 5433 {
		t.Errorf("Load() cfg.Database.Port = %v, want 5433", cfg.Database.Port)
	}

	if cfg.Database.User != "env-user" {
		t.Errorf("Load() cfg.Database.User = %v, want env-user", cfg.Database.User)
	}

	if cfg.Server.Host != "env-server" {
		t.Errorf("Load() cfg.Server.Host = %v, want env-server", cfg.Server.Host)
	}

	if cfg.Server.Port != 9000 {
		t.Errorf("Load() cfg.Server.Port = %v, want 9000", cfg.Server.Port)
	}

	if !cfg.Debug {
		t.Error("Load() cfg.Debug = false, want true")
	}
}

func TestParseBool(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want bool
	}{
		{"true", "true", true},
		{"True", "True", true},
		{"TRUE", "TRUE", true},
		{"1", "1", true},
		{"yes", "yes", true},
		{"YES", "YES", true},
		{"false", "false", false},
		{"False", "False", false},
		{"0", "0", false},
		{"no", "no", false},
		{"empty", "", false},
		{"with spaces", "  true  ", true},
		{"invalid", "invalid", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseBool(tt.s)
			if got != tt.want {
				t.Errorf("parseBool(%q) = %v, want %v", tt.s, got, tt.want)
			}
		})
	}
}

