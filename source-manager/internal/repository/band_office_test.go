package repository_test

import (
	"context"
	"testing"

	"github.com/jonesrussell/north-cloud/source-manager/internal/models"
	"github.com/jonesrussell/north-cloud/source-manager/internal/repository"
	"github.com/jonesrussell/north-cloud/source-manager/internal/testhelpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupBandOfficeTestDB(t *testing.T) (
	bandOfficeRepo *repository.BandOfficeRepository,
	communityRepo *repository.CommunityRepository,
	cleanup func(),
) {
	t.Helper()
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	db, dbCleanup := setupTestDB(t)
	logger := testhelpers.NewTestLogger()
	bandOfficeRepo = repository.NewBandOfficeRepository(db, logger)
	communityRepo = repository.NewCommunityRepository(db, logger)

	cleanup = func() {
		ctx := context.Background()
		_, _ = db.ExecContext(ctx, "TRUNCATE TABLE band_offices, communities CASCADE")
		dbCleanup()
	}

	return bandOfficeRepo, communityRepo, cleanup
}

func newTestBandOffice(communityID string) *models.BandOffice {
	return &models.BandOffice{
		CommunityID: communityID,
		DataSource:  "manual",
	}
}

func TestBandOfficeRepository_Create(t *testing.T) {
	boRepo, communityRepo, cleanup := setupBandOfficeTestDB(t)
	defer cleanup()

	ctx := context.Background()
	community := createTestCommunity(t, ctx, communityRepo)

	t.Run("create valid band office", func(t *testing.T) {
		bo := newTestBandOffice(community.ID)
		phone := "705-555-1234"
		bo.Phone = &phone
		city := "Sudbury"
		bo.City = &city

		err := boRepo.Create(ctx, bo)
		require.NoError(t, err)
		assert.NotEmpty(t, bo.ID)
		assert.False(t, bo.CreatedAt.IsZero())
	})

	t.Run("duplicate community_id returns error", func(t *testing.T) {
		bo := newTestBandOffice(community.ID)
		err := boRepo.Create(ctx, bo)
		require.Error(t, err)
	})
}

func TestBandOfficeRepository_GetByCommunity(t *testing.T) {
	boRepo, communityRepo, cleanup := setupBandOfficeTestDB(t)
	defer cleanup()

	ctx := context.Background()
	community := createTestCommunity(t, ctx, communityRepo)

	t.Run("found", func(t *testing.T) {
		bo := newTestBandOffice(community.ID)
		phone := "705-555-1234"
		bo.Phone = &phone
		require.NoError(t, boRepo.Create(ctx, bo))

		found, err := boRepo.GetByCommunity(ctx, community.ID)
		require.NoError(t, err)
		assert.Equal(t, community.ID, found.CommunityID)
		assert.Equal(t, "705-555-1234", *found.Phone)
	})

	t.Run("not found", func(t *testing.T) {
		found, err := boRepo.GetByCommunity(ctx, "nonexistent-id")
		require.NoError(t, err)
		assert.Nil(t, found)
	})
}

func TestBandOfficeRepository_Update(t *testing.T) {
	boRepo, communityRepo, cleanup := setupBandOfficeTestDB(t)
	defer cleanup()

	ctx := context.Background()
	community := createTestCommunity(t, ctx, communityRepo)
	bo := newTestBandOffice(community.ID)
	require.NoError(t, boRepo.Create(ctx, bo))

	email := "office@band.ca"
	bo.Email = &email
	hours := "Mon-Fri 8:30am-4:30pm"
	bo.OfficeHours = &hours

	err := boRepo.Update(ctx, bo)
	require.NoError(t, err)

	found, getErr := boRepo.GetByCommunity(ctx, community.ID)
	require.NoError(t, getErr)
	assert.Equal(t, "office@band.ca", *found.Email)
	assert.Equal(t, "Mon-Fri 8:30am-4:30pm", *found.OfficeHours)
}

func TestBandOfficeRepository_DeleteByCommunity(t *testing.T) {
	boRepo, communityRepo, cleanup := setupBandOfficeTestDB(t)
	defer cleanup()

	ctx := context.Background()
	community := createTestCommunity(t, ctx, communityRepo)
	bo := newTestBandOffice(community.ID)
	require.NoError(t, boRepo.Create(ctx, bo))

	err := boRepo.DeleteByCommunity(ctx, community.ID)
	require.NoError(t, err)

	found, getErr := boRepo.GetByCommunity(ctx, community.ID)
	require.NoError(t, getErr)
	assert.Nil(t, found)
}

func TestBandOfficeRepository_DeleteByCommunity_NotFound(t *testing.T) {
	boRepo, _, cleanup := setupBandOfficeTestDB(t)
	defer cleanup()

	ctx := context.Background()
	err := boRepo.DeleteByCommunity(ctx, "nonexistent-id")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestBandOfficeRepository_Upsert(t *testing.T) {
	boRepo, communityRepo, cleanup := setupBandOfficeTestDB(t)
	defer cleanup()

	ctx := context.Background()
	community := createTestCommunity(t, ctx, communityRepo)

	t.Run("insert when new", func(t *testing.T) {
		bo := newTestBandOffice(community.ID)
		phone := "705-555-0001"
		bo.Phone = &phone

		err := boRepo.Upsert(ctx, bo)
		require.NoError(t, err)
		assert.NotEmpty(t, bo.ID)
	})

	t.Run("update when exists", func(t *testing.T) {
		bo := newTestBandOffice(community.ID)
		phone := "705-555-9999"
		bo.Phone = &phone
		email := "new@band.ca"
		bo.Email = &email

		err := boRepo.Upsert(ctx, bo)
		require.NoError(t, err)

		found, getErr := boRepo.GetByCommunity(ctx, community.ID)
		require.NoError(t, getErr)
		assert.Equal(t, "705-555-9999", *found.Phone)
		assert.Equal(t, "new@band.ca", *found.Email)
	})
}
