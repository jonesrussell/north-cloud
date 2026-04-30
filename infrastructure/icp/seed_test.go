package icp_test

import (
	"testing"

	"github.com/jonesrussell/north-cloud/infrastructure/icp"
	"github.com/stretchr/testify/require"
)

func TestMatchRequiresCanadianAnchorForIndigenousChannel(t *testing.T) {
	seed := testSeed(t)

	result := icp.Match(seed, icp.Document{
		Title:  "ABC Indigenous reports new Aboriginal business program in Australia",
		Body:   "The Aboriginal and Torres Strait Islander program supports economic development.",
		Topics: []string{"indigenous"},
	})

	require.Nil(t, result)
}

func TestMatchEmitsSegmentsWithModelVersion(t *testing.T) {
	seed := testSeed(t)

	result := icp.Match(seed, icp.Document{
		Title:      "Wahnapitae First Nation selects Sudbury engineering consultancy",
		Body:       "The Indigenous-owned firm will support water infrastructure and economic development.",
		SourceName: "Northern Ontario Business",
		Topics:     []string{"indigenous", "mining"},
	})

	require.NotNil(t, result)
	require.Equal(t, icp.ModelVersionV1, result.ModelVersion)
	require.NotEmpty(t, result.Segments)
	require.Equal(t, "indigenous_channel", result.Segments[0].Segment)
	require.Contains(t, result.Segments[0].MatchedKeywords, "first nation")
}

func testSeed(t *testing.T) *icp.Seed {
	t.Helper()
	seed := &icp.Seed{
		SegmentSchemaVersion: 1,
		SeedUpdatedAt:        "2026-04-26",
		Segments: []icp.Segment{
			{
				Name:        "indigenous_channel",
				Description: "Indigenous-owned or adjacent organizations in Canada.",
				Keywords:    []string{"first nation", "indigenous-owned", "economic development"},
				Topics:      []string{"indigenous"},
				RequiredAny: []string{"first nation", "sudbury", "ontario", "canada"},
				MinScore:    0.30,
			},
			{
				Name:        "northern_ontario_industry",
				Description: "Northern Ontario industry.",
				Keywords:    []string{"sudbury", "mining"},
				MinScore:    0.30,
			},
			{
				Name:        "private_sector_smb",
				Description: "Canadian SMB.",
				Keywords:    []string{"consultancy"},
				MinScore:    0.30,
			},
		},
	}
	require.NoError(t, icp.ValidateSeed(seed))
	return seed
}
