package services_test

import (
	"testing"

	"github.com/jonesrussell/north-cloud/source-manager/internal/services"
	"github.com/stretchr/testify/assert"
)

func TestSafeFloat_NilPointer(t *testing.T) {
	t.Parallel()
	result := services.SafeFloat(nil)
	assert.InDelta(t, 0.0, result, 0.0001)
}

func TestSafeFloat_ZeroValue(t *testing.T) {
	t.Parallel()
	val := 0.0
	result := services.SafeFloat(&val)
	assert.InDelta(t, 0.0, result, 0.0001)
}

func TestSafeFloat_PositiveValue(t *testing.T) {
	t.Parallel()
	val := 42.5
	result := services.SafeFloat(&val)
	assert.InDelta(t, 42.5, result, 0.0001)
}

func TestSafeFloat_NegativeValue(t *testing.T) {
	t.Parallel()
	val := -15.75
	result := services.SafeFloat(&val)
	assert.InDelta(t, -15.75, result, 0.0001)
}

func TestSafeFloat_LargeValue(t *testing.T) {
	t.Parallel()
	val := 1e18
	result := services.SafeFloat(&val)
	assert.InDelta(t, 1e18, result, 1e12)
}

func TestSafeFloat_SmallValue(t *testing.T) {
	t.Parallel()
	val := 1e-10
	result := services.SafeFloat(&val)
	assert.InDelta(t, 1e-10, result, 1e-15)
}

func TestNewTravelTimeService(t *testing.T) {
	t.Parallel()
	// Verify constructor does not panic with nil args
	svc := services.NewTravelTimeService(nil, nil, nil, nil)
	assert.NotNil(t, svc)
}
