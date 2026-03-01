package database_test

import (
	"testing"

	"github.com/jonesrussell/north-cloud/social-publisher/internal/database"
	"github.com/stretchr/testify/assert"
)

func TestNewRepository_NotNil(t *testing.T) {
	repo := database.NewRepository(nil)
	assert.NotNil(t, repo)
}
