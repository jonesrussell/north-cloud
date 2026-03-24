package bootstrap_test

import (
	"testing"

	"github.com/jonesrussell/north-cloud/crawler/internal/bootstrap"
	dbconfig "github.com/jonesrussell/north-cloud/crawler/internal/config/database"
)

func TestDatabaseConfigFromInterface(t *testing.T) {
	t.Parallel()

	input := &dbconfig.Config{
		Host:     "dbhost",
		Port:     "5432",
		User:     "dbuser",
		Password: "dbpass",
		DBName:   "testdb",
		SSLMode:  "disable",
	}

	result := bootstrap.DatabaseConfigFromInterface(input)

	if result.Host != "dbhost" {
		t.Errorf("expected Host 'dbhost', got %q", result.Host)
	}
	if result.Port != "5432" {
		t.Errorf("expected Port '5432', got %q", result.Port)
	}
	if result.User != "dbuser" {
		t.Errorf("expected User 'dbuser', got %q", result.User)
	}
	if result.Password != "dbpass" {
		t.Errorf("expected Password 'dbpass', got %q", result.Password)
	}
	if result.DBName != "testdb" {
		t.Errorf("expected DBName 'testdb', got %q", result.DBName)
	}
	if result.SSLMode != "disable" {
		t.Errorf("expected SSLMode 'disable', got %q", result.SSLMode)
	}
}

func TestDatabaseConfigFromInterface_EmptyValues(t *testing.T) {
	t.Parallel()

	input := &dbconfig.Config{}
	result := bootstrap.DatabaseConfigFromInterface(input)

	if result.Host != "" {
		t.Errorf("expected empty Host, got %q", result.Host)
	}
	if result.Port != "" {
		t.Errorf("expected empty Port, got %q", result.Port)
	}
}
