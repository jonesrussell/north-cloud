package scope_test

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jonesrussell/north-cloud/alert-crawler/internal/domain"
	"github.com/jonesrussell/north-cloud/alert-crawler/internal/scope"
)

func makeSource(defaultScope []string) domain.AlertSource {
	return domain.AlertSource{
		DefaultScope: defaultScope,
	}
}

func TestResolve_DefaultsOnly(t *testing.T) {
	r := scope.New()
	src := makeSource([]string{"treaty:1", "canada:manitoba"})
	got := r.Resolve(src, "")
	assert.Equal(t, []string{"treaty:1", "canada:manitoba"}, got)
}

func TestResolve_LocationAddsCityAndAncestors(t *testing.T) {
	r := scope.New()
	src := makeSource([]string{"canada:manitoba"})
	got := r.Resolve(src, "Winnipeg")

	require.NotEmpty(t, got)
	assert.True(t, slices.Contains(got, "canada:manitoba"), "should contain canada:manitoba")
	assert.True(t, slices.Contains(got, "canada:manitoba:winnipeg"), "should contain city slug")
	assert.True(t, slices.Contains(got, "canada"), "should contain country slug")
}

func TestResolve_DeduplicatesParents(t *testing.T) {
	r := scope.New()
	// defaults include both province and country; city walk would add them again
	src := makeSource([]string{"canada:manitoba", "canada"})
	got := r.Resolve(src, "Winnipeg")

	// expected unique slugs: canada:manitoba, canada, canada:manitoba:winnipeg
	const wantLen = 3
	assert.Len(t, got, wantLen, "duplicates must be removed")
	assert.True(t, slices.Contains(got, "canada:manitoba:winnipeg"))
	assert.True(t, slices.Contains(got, "canada:manitoba"))
	assert.True(t, slices.Contains(got, "canada"))
}

func TestResolve_UnknownLocationNoOp(t *testing.T) {
	r := scope.New()
	src := makeSource([]string{"canada:manitoba"})
	got := r.Resolve(src, "Atlantis")
	assert.Equal(t, []string{"canada:manitoba"}, got)
}

func TestResolve_LowerCaseAndTrim(t *testing.T) {
	r := scope.New()
	src := makeSource([]string{})
	got := r.Resolve(src, "  WINNIPEG  ")
	assert.True(t, slices.Contains(got, "canada:manitoba:winnipeg"),
		"city slug should be present after normalisation")
}

func TestResolve_EmptySource(t *testing.T) {
	r := scope.New()
	src := makeSource(nil)
	got := r.Resolve(src, "")
	assert.Empty(t, got)
}
