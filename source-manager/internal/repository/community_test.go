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

func setupCommunityTestDB(t *testing.T) (repo *repository.CommunityRepository, cleanup func()) {
	t.Helper()
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	db, dbCleanup := setupTestDB(t)
	logger := testhelpers.NewTestLogger()
	repo = repository.NewCommunityRepository(db, logger)

	cleanup = func() {
		ctx := context.Background()
		_, _ = db.ExecContext(ctx, "TRUNCATE TABLE communities CASCADE")
		dbCleanup()
	}

	return repo, cleanup
}

func newTestCommunity(name, slug, communityType string) *models.Community {
	return &models.Community{
		Name:          name,
		Slug:          slug,
		CommunityType: communityType,
		DataSource:    "manual",
		Enabled:       true,
	}
}

func TestCommunityRepository_Create(t *testing.T) {
	repo, cleanup := setupCommunityTestDB(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("create valid community", func(t *testing.T) {
		c := newTestCommunity("Sudbury", "sudbury", "city")
		province := "ON"
		c.Province = &province

		err := repo.Create(ctx, c)
		require.NoError(t, err)
		assert.NotEmpty(t, c.ID)
		assert.False(t, c.CreatedAt.IsZero())
	})

	t.Run("duplicate slug returns error", func(t *testing.T) {
		c := newTestCommunity("Sudbury Duplicate", "sudbury", "city")
		err := repo.Create(ctx, c)
		assert.Error(t, err)
	})
}

func TestCommunityRepository_GetByID(t *testing.T) {
	repo, cleanup := setupCommunityTestDB(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("found", func(t *testing.T) {
		c := newTestCommunity("Get Test", "get-test", "city")
		require.NoError(t, repo.Create(ctx, c))

		found, err := repo.GetByID(ctx, c.ID)
		require.NoError(t, err)
		assert.Equal(t, c.ID, found.ID)
		assert.Equal(t, "Get Test", found.Name)
	})

	t.Run("not found", func(t *testing.T) {
		found, err := repo.GetByID(ctx, "nonexistent-id")
		require.NoError(t, err)
		assert.Nil(t, found)
	})
}

func TestCommunityRepository_GetBySlug(t *testing.T) {
	repo, cleanup := setupCommunityTestDB(t)
	defer cleanup()

	ctx := context.Background()

	c := newTestCommunity("Slug Test", "slug-test", "town")
	require.NoError(t, repo.Create(ctx, c))

	t.Run("found", func(t *testing.T) {
		found, err := repo.GetBySlug(ctx, "slug-test")
		require.NoError(t, err)
		assert.Equal(t, c.ID, found.ID)
	})

	t.Run("not found", func(t *testing.T) {
		found, err := repo.GetBySlug(ctx, "no-such-slug")
		require.NoError(t, err)
		assert.Nil(t, found)
	})
}

func TestCommunityRepository_Update(t *testing.T) {
	repo, cleanup := setupCommunityTestDB(t)
	defer cleanup()

	ctx := context.Background()
	c := newTestCommunity("Before Update", "before-update", "city")
	require.NoError(t, repo.Create(ctx, c))

	c.Name = "After Update"
	pop := 50000
	c.Population = &pop

	err := repo.Update(ctx, c)
	require.NoError(t, err)

	found, getErr := repo.GetByID(ctx, c.ID)
	require.NoError(t, getErr)
	assert.Equal(t, "After Update", found.Name)
	assert.Equal(t, 50000, *found.Population)
}

func TestCommunityRepository_Delete(t *testing.T) {
	repo, cleanup := setupCommunityTestDB(t)
	defer cleanup()

	ctx := context.Background()
	c := newTestCommunity("Delete Me", "delete-me", "town")
	require.NoError(t, repo.Create(ctx, c))

	err := repo.Delete(ctx, c.ID)
	require.NoError(t, err)

	found, getErr := repo.GetByID(ctx, c.ID)
	require.NoError(t, getErr)
	assert.Nil(t, found)
}

func TestCommunityRepository_CountAndList(t *testing.T) {
	repo, cleanup := setupCommunityTestDB(t)
	defer cleanup()

	ctx := context.Background()

	require.NoError(t, repo.Create(ctx, newTestCommunity("City A", "city-a", "city")))
	fnCommunity := newTestCommunity("FN B", "fn-b", "first_nation")
	province := "ON"
	fnCommunity.Province = &province
	require.NoError(t, repo.Create(ctx, fnCommunity))
	require.NoError(t, repo.Create(ctx, newTestCommunity("Town C", "town-c", "town")))

	t.Run("count all", func(t *testing.T) {
		count, err := repo.Count(ctx, models.CommunityFilter{})
		require.NoError(t, err)
		assert.Equal(t, 3, count)
	})

	t.Run("count by type", func(t *testing.T) {
		count, err := repo.Count(ctx, models.CommunityFilter{Type: "first_nation"})
		require.NoError(t, err)
		assert.Equal(t, 1, count)
	})

	t.Run("list paginated", func(t *testing.T) {
		results, err := repo.ListPaginated(ctx, models.CommunityFilter{Limit: 2})
		require.NoError(t, err)
		assert.Len(t, results, 2)
	})

	t.Run("list by province", func(t *testing.T) {
		results, err := repo.ListPaginated(ctx, models.CommunityFilter{Province: "ON"})
		require.NoError(t, err)
		assert.Len(t, results, 1)
		assert.Equal(t, "FN B", results[0].Name)
	})

	t.Run("search by name", func(t *testing.T) {
		results, err := repo.ListPaginated(ctx, models.CommunityFilter{Search: "city"})
		require.NoError(t, err)
		assert.Len(t, results, 1)
		assert.Equal(t, "City A", results[0].Name)
	})
}

func TestCommunityRepository_UpsertByInacID(t *testing.T) {
	repo, cleanup := setupCommunityTestDB(t)
	defer cleanup()

	ctx := context.Background()
	inacID := "123"

	t.Run("insert when new", func(t *testing.T) {
		c := newTestCommunity("FN One", "fn-one", "first_nation")
		c.InacID = &inacID
		c.DataSource = "cirnac"

		err := repo.UpsertByInacID(ctx, c)
		require.NoError(t, err)
		assert.NotEmpty(t, c.ID)
	})

	t.Run("update when exists", func(t *testing.T) {
		c := newTestCommunity("FN One Updated", "fn-one", "first_nation")
		c.InacID = &inacID
		c.DataSource = "cirnac"

		err := repo.UpsertByInacID(ctx, c)
		require.NoError(t, err)

		found, getErr := repo.GetBySlug(ctx, "fn-one")
		require.NoError(t, getErr)
		assert.Equal(t, "FN One Updated", found.Name)
	})
}

func TestCommunityRepository_UpsertByStatCanCSD(t *testing.T) {
	repo, cleanup := setupCommunityTestDB(t)
	defer cleanup()

	ctx := context.Background()
	csd := "3553005"

	t.Run("insert when new", func(t *testing.T) {
		c := newTestCommunity("Sudbury CSD", "sudbury-csd", "city")
		c.StatCanCSD = &csd
		c.DataSource = "statscan"

		err := repo.UpsertByStatCanCSD(ctx, c)
		require.NoError(t, err)
		assert.NotEmpty(t, c.ID)
	})

	t.Run("update when exists", func(t *testing.T) {
		pop := 165000
		c := newTestCommunity("Greater Sudbury", "sudbury-csd", "city")
		c.StatCanCSD = &csd
		c.Population = &pop
		c.DataSource = "statscan"

		err := repo.UpsertByStatCanCSD(ctx, c)
		require.NoError(t, err)

		found, getErr := repo.GetBySlug(ctx, "sudbury-csd")
		require.NoError(t, getErr)
		assert.Equal(t, "Greater Sudbury", found.Name)
		assert.Equal(t, 165000, *found.Population)
	})
}

func TestCommunityRepository_FindNearby(t *testing.T) {
	repo, cleanup := setupCommunityTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Seed: Sudbury (46.49, -81.00), Timmins (48.47, -81.33), Toronto (43.65, -79.38)
	sudbury := newTestCommunity("Sudbury", "sudbury-nearby", "city")
	sudLat, sudLon := 46.49, -81.00
	sudbury.Latitude = &sudLat
	sudbury.Longitude = &sudLon
	require.NoError(t, repo.Create(ctx, sudbury))

	timmins := newTestCommunity("Timmins", "timmins-nearby", "city")
	timLat, timLon := 48.47, -81.33
	timmins.Latitude = &timLat
	timmins.Longitude = &timLon
	require.NoError(t, repo.Create(ctx, timmins))

	toronto := newTestCommunity("Toronto", "toronto-nearby", "city")
	torLat, torLon := 43.65, -79.38
	toronto.Latitude = &torLat
	toronto.Longitude = &torLon
	require.NoError(t, repo.Create(ctx, toronto))

	noCoords := newTestCommunity("No Coords", "no-coords-nearby", "town")
	require.NoError(t, repo.Create(ctx, noCoords))

	t.Run("finds within radius sorted by distance", func(t *testing.T) {
		results, err := repo.FindNearby(ctx, 46.49, -81.00, 250, 10)
		require.NoError(t, err)
		assert.Len(t, results, 2)
		assert.Equal(t, "Sudbury", results[0].Name)
		assert.Equal(t, "Timmins", results[1].Name)
		assert.Less(t, results[0].DistanceKm, results[1].DistanceKm)
	})

	t.Run("respects limit", func(t *testing.T) {
		results, err := repo.FindNearby(ctx, 46.49, -81.00, 500, 1)
		require.NoError(t, err)
		assert.Len(t, results, 1)
	})

	t.Run("empty when nothing nearby", func(t *testing.T) {
		results, err := repo.FindNearby(ctx, 40.00, -40.00, 100, 10)
		require.NoError(t, err)
		assert.Empty(t, results)
	})
}
