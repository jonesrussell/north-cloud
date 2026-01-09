// Package config provides a unified configuration loader for all North Cloud services.
// It uses YAML files with environment variable overrides.
//
// Environment Variables and .env Files:
//
// The package automatically loads .env files before applying environment variable overrides.
// Files are loaded in the following priority order (higher priority overrides lower):
//
//  1. Environment variable ENV_FILE (if set, loads only this file)
//  2. .env.local (if exists, overrides .env)
//  3. .env (default, always checked if ENV_FILE is not set)
//
// Example .env file:
//
//	MY_PORT=8080
//	MY_HOST=localhost
//	DEBUG=true
//
// Example config struct:
//
//	type MyConfig struct {
//	    Port  int    `yaml:"port" env:"MY_PORT"`
//	    Host  string `yaml:"host" env:"MY_HOST"`
//	    Debug bool   `yaml:"debug" env:"DEBUG"`
//	}
//
//	cfg, err := config.Load[MyConfig]("config.yml")
package config

import (
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"gopkg.in/yaml.v3"
)

// loadEnvFiles loads .env files in priority order:
// 1. ENV_FILE environment variable (if set, loads only this file)
// 2. .env.local (if exists, overrides .env)
// 3. .env (default)
// Returns error only if loading fails (file not found errors are ignored).
func loadEnvFiles() error {
	// Check for ENV_FILE environment variable (highest priority)
	if envFile := os.Getenv("ENV_FILE"); envFile != "" {
		if err := godotenv.Load(envFile); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("load env file %s: %w", envFile, err)
		}
		return nil
	}

	// Load .env.local if it exists (overrides .env)
	if err := godotenv.Load(".env.local"); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("load .env.local: %w", err)
	}

	// Load .env (default, always checked)
	if err := godotenv.Load(".env"); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("load .env: %w", err)
	}

	return nil
}

// Load reads a YAML config file and applies environment variable overrides.
// The type parameter T must be a struct type.
// Environment variables are specified using the `env` struct tag.
//
// Load automatically loads .env files before applying environment variable overrides.
// See package documentation for .env file loading priority.
//
// Example:
//
//	type MyConfig struct {
//	    Port int    `yaml:"port" env:"MY_PORT"`
//	    Host string `yaml:"host" env:"MY_HOST"`
//	}
//
//	cfg, err := config.Load[MyConfig]("config.yml")
func Load[T any](path string) (*T, error) {
	// Load .env files first (non-fatal if files don't exist)
	if err := loadEnvFiles(); err != nil {
		return nil, fmt.Errorf("load environment files: %w", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file %s: %w", path, err)
	}

	var cfg T
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	applyEnvOverrides(&cfg)
	return &cfg, nil
}

// LoadWithDefaults reads a YAML config file, applies defaults, then applies env overrides.
// The defaults function is called before environment variable overrides are applied.
func LoadWithDefaults[T any](path string, setDefaults func(*T)) (*T, error) {
	cfg, err := Load[T](path)
	if err != nil {
		return nil, err
	}

	if setDefaults != nil {
		setDefaults(cfg)
	}

	// Re-apply env overrides after defaults (env always wins)
	applyEnvOverrides(cfg)
	return cfg, nil
}

// MustLoad is like Load but panics if an error occurs.
// Use this for initialization where failure should be fatal.
func MustLoad[T any](path string) *T {
	cfg, err := Load[T](path)
	if err != nil {
		panic(fmt.Sprintf("failed to load config: %v", err))
	}
	return cfg
}

// applyEnvOverrides uses struct tags to apply environment variable values.
// Tag format: `env:"VAR_NAME"`
func applyEnvOverrides(cfg any) {
	v := reflect.ValueOf(cfg)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	applyEnvToStruct(v)
}

func applyEnvToStruct(v reflect.Value) {
	if v.Kind() != reflect.Struct {
		return
	}

	t := v.Type()
	for i := range v.NumField() {
		field := v.Field(i)
		fieldType := t.Field(i)

		// Skip unexported fields
		if !field.CanSet() {
			continue
		}

		// Recursively handle embedded/nested structs
		if field.Kind() == reflect.Struct {
			applyEnvToStruct(field)
			continue
		}

		// Handle pointer to struct
		if field.Kind() == reflect.Ptr && field.Type().Elem().Kind() == reflect.Struct {
			if field.IsNil() {
				// Initialize nil pointer
				field.Set(reflect.New(field.Type().Elem()))
			}
			applyEnvToStruct(field.Elem())
			continue
		}

		envTag := fieldType.Tag.Get("env")
		if envTag == "" {
			continue
		}

		envVal := os.Getenv(envTag)
		if envVal == "" {
			continue
		}

		setFieldFromString(field, envVal)
	}
}

func setFieldFromString(field reflect.Value, val string) {
	switch field.Kind() {
	case reflect.String:
		field.SetString(val)

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		// Special handling for time.Duration
		if field.Type() == reflect.TypeOf(time.Duration(0)) {
			if d, err := time.ParseDuration(val); err == nil {
				field.SetInt(int64(d))
			}
		} else {
			if i, err := strconv.ParseInt(val, 10, 64); err == nil {
				field.SetInt(i)
			}
		}

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if u, err := strconv.ParseUint(val, 10, 64); err == nil {
			field.SetUint(u)
		}

	case reflect.Float32, reflect.Float64:
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			field.SetFloat(f)
		}

	case reflect.Bool:
		field.SetBool(parseBool(val))

	case reflect.Slice:
		if field.Type().Elem().Kind() == reflect.String {
			parts := strings.Split(val, ",")
			for i, p := range parts {
				parts[i] = strings.TrimSpace(p)
			}
			field.Set(reflect.ValueOf(parts))
		}
	}
}

// parseBool parses a string as a boolean.
// Returns true for "true", "1", "yes" (case-insensitive).
func parseBool(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	return s == "true" || s == "1" || s == "yes"
}

// GetConfigPath returns the config path from CONFIG_PATH env var or the default.
func GetConfigPath(defaultPath string) string {
	if path := os.Getenv("CONFIG_PATH"); path != "" {
		return path
	}
	return defaultPath
}
