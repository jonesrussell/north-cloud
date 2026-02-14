// publisher/internal/router/location_test.go
//
//nolint:testpackage // Testing internal router requires same package access
package router

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenerateLocationChannels_CrimeCanadianCity(t *testing.T) {
	t.Helper()

	article := &Article{
		ID:                  "loc-crime-city",
		CrimeRelevance:      "core_street_crime",
		LocationCity:        "sudbury",
		LocationProvince:    "ON",
		LocationCountry:     "canada",
		LocationSpecificity: "city",
	}

	channels := GenerateLocationChannels(article)

	assert.Contains(t, channels, "crime:local:sudbury")
	assert.Contains(t, channels, "crime:province:on")
	assert.Contains(t, channels, "crime:canada")
	assert.Len(t, channels, 3)
}

func TestGenerateLocationChannels_CrimeInternational(t *testing.T) {
	t.Helper()

	article := &Article{
		ID:                  "loc-crime-intl",
		CrimeRelevance:      "core_street_crime",
		LocationCountry:     "united_states",
		LocationSpecificity: "country",
	}

	channels := GenerateLocationChannels(article)

	assert.Equal(t, []string{"crime:international"}, channels)
}

func TestGenerateLocationChannels_CrimeProvinceOnly(t *testing.T) {
	t.Helper()

	article := &Article{
		ID:                  "loc-crime-province",
		CrimeRelevance:      "peripheral_crime",
		LocationProvince:    "BC",
		LocationCountry:     "canada",
		LocationSpecificity: "province",
	}

	channels := GenerateLocationChannels(article)

	assert.Contains(t, channels, "crime:province:bc")
	assert.Contains(t, channels, "crime:canada")
	assert.Len(t, channels, 2)
	// No local channel for province-level specificity
	for _, ch := range channels {
		assert.NotContains(t, ch, "crime:local:")
	}
}

func TestGenerateLocationChannels_MiningSkipped(t *testing.T) {
	t.Helper()

	// Mining location channels are handled by Layer 5 (GenerateMiningChannels),
	// so Layer 4 should not generate them.
	article := &Article{
		ID:                  "loc-mining-skip",
		Mining:              &MiningData{Relevance: "core_mining"},
		LocationCity:        "timmins",
		LocationProvince:    "ON",
		LocationCountry:     "canada",
		LocationSpecificity: "city",
	}

	channels := GenerateLocationChannels(article)

	assert.Empty(t, channels)
}

func TestGenerateLocationChannels_EntertainmentCanadianCity(t *testing.T) {
	t.Helper()

	article := &Article{
		ID:                  "loc-ent-city",
		Entertainment:       &EntertainmentData{Relevance: "core_entertainment"},
		LocationCity:        "toronto",
		LocationProvince:    "ON",
		LocationCountry:     "canada",
		LocationSpecificity: "city",
	}

	channels := GenerateLocationChannels(article)

	assert.Contains(t, channels, "entertainment:local:toronto")
	assert.Contains(t, channels, "entertainment:province:on")
	assert.Contains(t, channels, "entertainment:canada")
	assert.Len(t, channels, 3)
}

func TestGenerateLocationChannels_MultiTopic(t *testing.T) {
	t.Helper()

	article := &Article{
		ID:                  "loc-multi",
		CrimeRelevance:      "core_street_crime",
		Entertainment:       &EntertainmentData{Relevance: "core_entertainment"},
		LocationCity:        "toronto",
		LocationProvince:    "ON",
		LocationCountry:     "canada",
		LocationSpecificity: "city",
	}

	channels := GenerateLocationChannels(article)

	// Crime channels
	assert.Contains(t, channels, "crime:local:toronto")
	assert.Contains(t, channels, "crime:province:on")
	assert.Contains(t, channels, "crime:canada")
	// Entertainment channels
	assert.Contains(t, channels, "entertainment:local:toronto")
	assert.Contains(t, channels, "entertainment:province:on")
	assert.Contains(t, channels, "entertainment:canada")
	assert.Len(t, channels, 6)
}

func TestGenerateLocationChannels_NoActiveTopic(t *testing.T) {
	t.Helper()

	article := &Article{
		ID:                  "loc-no-topic",
		CrimeRelevance:      "not_crime",
		LocationCity:        "vancouver",
		LocationProvince:    "BC",
		LocationCountry:     "canada",
		LocationSpecificity: "city",
	}

	channels := GenerateLocationChannels(article)

	assert.Empty(t, channels)
}

func TestGenerateLocationChannels_UnknownLocation(t *testing.T) {
	t.Helper()

	article := &Article{
		ID:                  "loc-unknown",
		CrimeRelevance:      "core_street_crime",
		LocationCountry:     "unknown",
		LocationSpecificity: "unknown",
	}

	channels := GenerateLocationChannels(article)

	assert.Empty(t, channels)
}

func TestGenerateLocationChannels_EmptyLocationCountry(t *testing.T) {
	t.Helper()

	article := &Article{
		ID:                  "loc-empty-country",
		CrimeRelevance:      "core_street_crime",
		LocationCity:        "toronto",
		LocationProvince:    "ON",
		LocationCountry:     "",
		LocationSpecificity: "city",
	}

	channels := GenerateLocationChannels(article)

	assert.Empty(t, channels)
}

func TestGenerateLocationChannels_EntertainmentNotEntertainment(t *testing.T) {
	t.Helper()

	article := &Article{
		ID:                  "loc-ent-not",
		Entertainment:       &EntertainmentData{Relevance: "not_entertainment"},
		LocationCity:        "toronto",
		LocationProvince:    "ON",
		LocationCountry:     "canada",
		LocationSpecificity: "city",
	}

	channels := GenerateLocationChannels(article)

	assert.Empty(t, channels)
}
