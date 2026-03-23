package config_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jonesrussell/north-cloud/ircd/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_ValidConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yml")
	err := os.WriteFile(path, []byte(`
server:
  name: test.irc
  network: TestNet
  listen: "127.0.0.1:6667"
`), 0644)
	require.NoError(t, err)

	cfg, err := config.Load(path)
	require.NoError(t, err)
	assert.Equal(t, "test.irc", cfg.Server.Name)
	assert.Equal(t, "TestNet", cfg.Server.Network)
	assert.Equal(t, "127.0.0.1:6667", cfg.Server.Listen)
}

func TestLoad_Defaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yml")
	err := os.WriteFile(path, []byte("server:\n  name: test.irc\n"), 0644)
	require.NoError(t, err)

	cfg, err := config.Load(path)
	require.NoError(t, err)
	assert.Equal(t, "127.0.0.1:6667", cfg.Server.Listen)
	assert.Equal(t, 256, cfg.Server.MaxClients)
	assert.Equal(t, 90*time.Second, cfg.Server.PingInterval)
	assert.Equal(t, 120*time.Second, cfg.Server.PongTimeout)
}

func TestLoad_MissingName(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yml")
	err := os.WriteFile(path, []byte("server:\n  listen: ':6667'\n"), 0644)
	require.NoError(t, err)

	_, err = config.Load(path)
	assert.Error(t, err)
}

func TestLoad_MissingFile(t *testing.T) {
	_, err := config.Load("/nonexistent/config.yml")
	assert.Error(t, err)
}
