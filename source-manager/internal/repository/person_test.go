package repository_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/google/uuid"
	"github.com/jonesrussell/north-cloud/source-manager/internal/models"
	"github.com/jonesrussell/north-cloud/source-manager/internal/repository"
	"github.com/jonesrussell/north-cloud/source-manager/internal/testhelpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupPersonTestDB(t *testing.T) (
	personRepo *repository.PersonRepository,
	communityRepo *repository.CommunityRepository,
	db *sql.DB,
	cleanup func(),
) {
	t.Helper()
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	var dbCleanup func()
	db, dbCleanup = setupTestDB(t)
	logger := testhelpers.NewTestLogger()
	personRepo = repository.NewPersonRepository(db, logger)
	communityRepo = repository.NewCommunityRepository(db, logger)

	cleanup = func() {
		ctx := context.Background()
		_, _ = db.ExecContext(ctx, "TRUNCATE TABLE people_history, people, communities CASCADE")
		dbCleanup()
	}

	return personRepo, communityRepo, db, cleanup
}

func createTestCommunity(t *testing.T, ctx context.Context, repo *repository.CommunityRepository) *models.Community {
	t.Helper()
	suffix := uuid.New().String()
	community := newTestCommunity("Test Community "+suffix, "test-community-"+suffix, "first_nation")
	require.NoError(t, repo.Create(ctx, community))
	return community
}

func newTestPerson(communityID, name, slug, role string) *models.Person {
	return &models.Person{
		CommunityID: communityID,
		Name:        name,
		Slug:        slug,
		Role:        role,
		DataSource:  "manual",
		IsCurrent:   true,
	}
}

func TestPersonRepository_Create(t *testing.T) {
	personRepo, communityRepo, _, cleanup := setupPersonTestDB(t)
	defer cleanup()

	ctx := context.Background()
	community := createTestCommunity(t, ctx, communityRepo)

	t.Run("create valid person", func(t *testing.T) {
		p := newTestPerson(community.ID, "Chief Test", "chief-test", "chief")
		roleTitle := "Chief Executive"
		p.RoleTitle = &roleTitle

		err := personRepo.Create(ctx, p)
		require.NoError(t, err)
		assert.NotEmpty(t, p.ID)
		assert.False(t, p.CreatedAt.IsZero())
		assert.True(t, p.IsCurrent)
	})
}

func TestPersonRepository_GetByID(t *testing.T) {
	personRepo, communityRepo, _, cleanup := setupPersonTestDB(t)
	defer cleanup()

	ctx := context.Background()
	community := createTestCommunity(t, ctx, communityRepo)

	t.Run("found", func(t *testing.T) {
		p := newTestPerson(community.ID, "Get Chief", "get-chief", "chief")
		require.NoError(t, personRepo.Create(ctx, p))

		found, err := personRepo.GetByID(ctx, p.ID)
		require.NoError(t, err)
		assert.Equal(t, p.ID, found.ID)
		assert.Equal(t, "Get Chief", found.Name)
	})

	t.Run("not found", func(t *testing.T) {
		found, err := personRepo.GetByID(ctx, "nonexistent-id")
		require.NoError(t, err)
		assert.Nil(t, found)
	})
}

func TestPersonRepository_Update(t *testing.T) {
	personRepo, communityRepo, _, cleanup := setupPersonTestDB(t)
	defer cleanup()

	ctx := context.Background()
	community := createTestCommunity(t, ctx, communityRepo)
	p := newTestPerson(community.ID, "Before Update", "before-update", "councillor")
	require.NoError(t, personRepo.Create(ctx, p))

	p.Name = "After Update"
	phone := "705-555-0000"
	p.Phone = &phone

	err := personRepo.Update(ctx, p)
	require.NoError(t, err)

	found, getErr := personRepo.GetByID(ctx, p.ID)
	require.NoError(t, getErr)
	assert.Equal(t, "After Update", found.Name)
	assert.Equal(t, "705-555-0000", *found.Phone)
}

func TestPersonRepository_Delete(t *testing.T) {
	personRepo, communityRepo, _, cleanup := setupPersonTestDB(t)
	defer cleanup()

	ctx := context.Background()
	community := createTestCommunity(t, ctx, communityRepo)
	p := newTestPerson(community.ID, "Delete Me", "delete-me", "councillor")
	require.NoError(t, personRepo.Create(ctx, p))

	err := personRepo.Delete(ctx, p.ID)
	require.NoError(t, err)

	found, getErr := personRepo.GetByID(ctx, p.ID)
	require.NoError(t, getErr)
	assert.Nil(t, found)
}

func TestPersonRepository_ListByCommunity(t *testing.T) {
	personRepo, communityRepo, _, cleanup := setupPersonTestDB(t)
	defer cleanup()

	ctx := context.Background()
	community := createTestCommunity(t, ctx, communityRepo)

	require.NoError(t, personRepo.Create(ctx, newTestPerson(community.ID, "P1", "p1", "chief")))
	require.NoError(t, personRepo.Create(ctx, newTestPerson(community.ID, "P2", "p2", "councillor")))
	p3 := newTestPerson(community.ID, "P3", "p3", "councillor")
	p3.IsCurrent = false
	require.NoError(t, personRepo.Create(ctx, p3))

	t.Run("list all for community", func(t *testing.T) {
		results, err := personRepo.ListByCommunity(ctx, models.PersonFilter{
			CommunityID: community.ID,
		})
		require.NoError(t, err)
		assert.Len(t, results, 3)
	})

	t.Run("filter by role", func(t *testing.T) {
		results, err := personRepo.ListByCommunity(ctx, models.PersonFilter{
			CommunityID: community.ID,
			Role:        "councillor",
		})
		require.NoError(t, err)
		assert.Len(t, results, 2)
	})

	t.Run("current only", func(t *testing.T) {
		results, err := personRepo.ListByCommunity(ctx, models.PersonFilter{
			CommunityID: community.ID,
			CurrentOnly: true,
		})
		require.NoError(t, err)
		assert.Len(t, results, 2)
	})

	t.Run("error when community_id empty", func(t *testing.T) {
		_, err := personRepo.ListByCommunity(ctx, models.PersonFilter{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "community_id is required")
	})

	t.Run("pagination", func(t *testing.T) {
		results, err := personRepo.ListByCommunity(ctx, models.PersonFilter{
			CommunityID: community.ID,
			Limit:       1,
			Offset:      0,
		})
		require.NoError(t, err)
		assert.Len(t, results, 1)
	})
}

func TestPersonRepository_Count(t *testing.T) {
	personRepo, communityRepo, _, cleanup := setupPersonTestDB(t)
	defer cleanup()

	ctx := context.Background()
	community := createTestCommunity(t, ctx, communityRepo)

	require.NoError(t, personRepo.Create(ctx, newTestPerson(community.ID, "P1", "p1", "chief")))
	require.NoError(t, personRepo.Create(ctx, newTestPerson(community.ID, "P2", "p2", "councillor")))

	t.Run("count all", func(t *testing.T) {
		count, err := personRepo.Count(ctx, models.PersonFilter{CommunityID: community.ID})
		require.NoError(t, err)
		assert.Equal(t, 2, count)
	})

	t.Run("count by role", func(t *testing.T) {
		count, err := personRepo.Count(ctx, models.PersonFilter{CommunityID: community.ID, Role: "chief"})
		require.NoError(t, err)
		assert.Equal(t, 1, count)
	})

	t.Run("error when community_id empty", func(t *testing.T) {
		_, err := personRepo.Count(ctx, models.PersonFilter{})
		require.Error(t, err)
	})
}

func TestPersonRepository_ArchiveTerm(t *testing.T) {
	personRepo, communityRepo, db, cleanup := setupPersonTestDB(t)
	defer cleanup()

	ctx := context.Background()
	community := createTestCommunity(t, ctx, communityRepo)

	t.Run("archives current person to history", func(t *testing.T) {
		p := newTestPerson(community.ID, "Old Chief", "old-chief", "chief")
		require.NoError(t, personRepo.Create(ctx, p))
		require.True(t, p.IsCurrent)

		err := personRepo.ArchiveTerm(ctx, p.ID)
		require.NoError(t, err)

		updated, getErr := personRepo.GetByID(ctx, p.ID)
		require.NoError(t, getErr)
		assert.False(t, updated.IsCurrent)
		assert.NotNil(t, updated.TermEnd)

		var history models.PersonHistory
		historyQuery := `SELECT id, person_id, community_id, name, role, term_start, term_end,
			data_source, source_url, archived_at FROM people_history WHERE person_id = $1`
		historyRow := db.QueryRowContext(ctx, historyQuery, p.ID)
		scanErr := historyRow.Scan(
			&history.ID, &history.PersonID, &history.CommunityID, &history.Name, &history.Role,
			&history.TermStart, &history.TermEnd, &history.DataSource, &history.SourceURL,
			&history.ArchivedAt,
		)
		require.NoError(t, scanErr)
		assert.Equal(t, p.ID, history.PersonID)
		assert.Equal(t, community.ID, history.CommunityID)
		assert.Equal(t, "Old Chief", history.Name)
		assert.Equal(t, "chief", history.Role)
		assert.False(t, history.ArchivedAt.IsZero())
	})

	t.Run("archiving already-archived person creates duplicate history", func(t *testing.T) {
		p := newTestPerson(community.ID, "Twice Archived", "twice-archived", "councillor")
		require.NoError(t, personRepo.Create(ctx, p))

		require.NoError(t, personRepo.ArchiveTerm(ctx, p.ID))

		err := personRepo.ArchiveTerm(ctx, p.ID)
		require.NoError(t, err)

		var count int
		countErr := db.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM people_history WHERE person_id = $1", p.ID).Scan(&count)
		require.NoError(t, countErr)
		assert.Equal(t, 2, count)
	})

	t.Run("error when person not found", func(t *testing.T) {
		err := personRepo.ArchiveTerm(ctx, "nonexistent-id")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}
