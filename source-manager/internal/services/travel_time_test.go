package services_test

import (
	"testing"

	"github.com/jonesrussell/north-cloud/source-manager/internal/services"
	"github.com/stretchr/testify/assert"
)

func TestSafeFloat(t *testing.T) {
	t.Parallel()

	t.Run("nil returns zero", func(t *testing.T) {
		t.Parallel()
		assert.InDelta(t, 0.0, services.SafeFloat(nil), 0.001)
	})

	t.Run("non-nil returns value", func(t *testing.T) {
		t.Parallel()
		val := 48.123
		assert.InDelta(t, 48.123, services.SafeFloat(&val), 0.001)
	})
}
