package icpstore_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
	"github.com/jonesrussell/north-cloud/source-manager/internal/icpstore"
	"github.com/stretchr/testify/require"
)

func TestStoreReloadsSeedWithoutRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "icp-segments.yml")
	require.NoError(t, os.WriteFile(path, []byte(testSeedYAML("2026-04-26")), 0o600))

	store, err := icpstore.New(path, 25*time.Millisecond, infralogger.NewNop())
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go store.Run(ctx)

	require.NoError(t, os.WriteFile(path, []byte(testSeedYAML("2026-04-27")), 0o600))

	require.Eventually(t, func() bool {
		return store.Current().SeedUpdatedAt == "2026-04-27"
	}, 3*time.Second, 25*time.Millisecond)
}

func testSeedYAML(updatedAt string) string {
	return `segment_schema_version: 1
seed_updated_at: "` + updatedAt + `"
segments:
  - name: indigenous_channel
    description: Indigenous channel.
    keywords: ["First Nation"]
    topics: ["indigenous"]
    required_any: ["Canada"]
    min_score: 0.3
  - name: northern_ontario_industry
    description: Northern Ontario industry.
    keywords: ["Sudbury"]
    topics: ["mining"]
    min_score: 0.3
  - name: private_sector_smb
    description: Canadian SMB.
    keywords: ["law firm"]
    topics: ["business"]
    min_score: 0.3
`
}
