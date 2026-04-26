package classifier

import (
	"context"
	"testing"

	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
	"github.com/jonesrussell/north-cloud/infrastructure/icp"
	"github.com/stretchr/testify/require"
)

func TestSectorAlignmentExtractorEmitsICPResult(t *testing.T) {
	extractor := NewSectorAlignmentExtractor(StaticICPSeedProvider{SeedValue: &icp.Seed{
		SegmentSchemaVersion: 1,
		SeedUpdatedAt:        "2026-04-26",
		Segments: []icp.Segment{
			{
				Name:        "indigenous_channel",
				Description: "Indigenous-owned Canadian organizations.",
				Keywords:    []string{"first nation", "indigenous-owned"},
				RequiredAny: []string{"first nation", "canada"},
				MinScore:    0.30,
			},
			{Name: "northern_ontario_industry", Description: "Northern Ontario industry.", Keywords: []string{"sudbury"}, MinScore: 0.30},
			{Name: "private_sector_smb", Description: "Canadian SMB.", Keywords: []string{"consulting firm"}, MinScore: 0.30},
		},
	}})

	result, err := extractor.Extract(context.Background(), &domain.RawContent{
		Title:   "Indigenous-owned consulting firm wins First Nation project",
		RawText: "The Canada-based team will support economic development.",
	}, []string{"business"})

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, icp.ModelVersionV1, result.ModelVersion)
	require.Equal(t, "indigenous_channel", result.Segments[0].Segment)
}
