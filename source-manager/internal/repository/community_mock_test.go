//nolint:testpackage // Testing with sqlmock requires same-package access
package repository

import (
	"context"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jonesrussell/north-cloud/source-manager/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newMockCommunityRepo(t *testing.T) (*CommunityRepository, sqlmock.Sqlmock, func()) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)

	repo := NewCommunityRepository(db, newTestLogger(t))
	cleanup := func() {
		mock.ExpectClose()
		db.Close()
	}

	return repo, mock, cleanup
}

func communityColumns() []string {
	return []string{
		"id", "name", "slug", "community_type", "province", "region",
		"inac_id", "statcan_csd", "osm_relation_id", "wikidata_qid", "latitude", "longitude",
		"nation", "treaty", "language_group", "reserve_name", "population", "population_year",
		"website", "feed_url", "data_source", "source_id", "enabled", "created_at", "updated_at",
	}
}

func TestCommunityRepository_Create_Mock(t *testing.T) {
	repo, mock, cleanup := newMockCommunityRepo(t)
	defer cleanup()

	ctx := context.Background()
	c := &models.Community{
		Name:          "Sudbury",
		Slug:          "sudbury",
		CommunityType: "city",
		DataSource:    "manual",
		Enabled:       true,
	}

	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO communities")).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err := repo.Create(ctx, c)
	require.NoError(t, err)
	assert.NotEmpty(t, c.ID)
	assert.False(t, c.CreatedAt.IsZero())

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCommunityRepository_GetByID_Mock(t *testing.T) {
	repo, mock, cleanup := newMockCommunityRepo(t)
	defer cleanup()

	ctx := context.Background()
	now := time.Now()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, name, slug")).
		WithArgs("c-1").
		WillReturnRows(
			sqlmock.NewRows(communityColumns()).AddRow(
				"c-1", "Sudbury", "sudbury", "city", "ON", nil,
				nil, nil, nil, nil, 46.49, -81.0,
				nil, nil, nil, nil, nil, nil,
				nil, nil, "manual", nil, true, now, now,
			),
		)

	c, err := repo.GetByID(ctx, "c-1")
	require.NoError(t, err)
	require.NotNil(t, c)
	assert.Equal(t, "Sudbury", c.Name)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCommunityRepository_GetByID_NotFound(t *testing.T) {
	repo, mock, cleanup := newMockCommunityRepo(t)
	defer cleanup()

	ctx := context.Background()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, name, slug")).
		WithArgs("missing").
		WillReturnRows(sqlmock.NewRows(communityColumns()))

	c, err := repo.GetByID(ctx, "missing")
	require.NoError(t, err)
	assert.Nil(t, c)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCommunityRepository_Delete_Mock(t *testing.T) {
	repo, mock, cleanup := newMockCommunityRepo(t)
	defer cleanup()

	ctx := context.Background()

	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM communities WHERE id = $1")).
		WithArgs("del-c").
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := repo.Delete(ctx, "del-c")
	require.NoError(t, err)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCommunityRepository_Delete_NotFound(t *testing.T) {
	repo, mock, cleanup := newMockCommunityRepo(t)
	defer cleanup()

	ctx := context.Background()

	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM communities WHERE id = $1")).
		WithArgs("missing").
		WillReturnResult(sqlmock.NewResult(0, 0))

	err := repo.Delete(ctx, "missing")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCommunityRepository_Count_Mock(t *testing.T) {
	repo, mock, cleanup := newMockCommunityRepo(t)
	defer cleanup()

	ctx := context.Background()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM communities")).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(15))

	count, err := repo.Count(ctx, models.CommunityFilter{})
	require.NoError(t, err)
	assert.Equal(t, 15, count)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCommunityRepository_ListRegions_Mock(t *testing.T) {
	repo, mock, cleanup := newMockCommunityRepo(t)
	defer cleanup()

	ctx := context.Background()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT COALESCE(province")).
		WillReturnRows(
			sqlmock.NewRows([]string{"province", "region", "count"}).
				AddRow("ON", "", 5).
				AddRow("BC", "", 3),
		)

	regions, err := repo.ListRegions(ctx)
	require.NoError(t, err)
	require.Len(t, regions, 2)
	assert.Equal(t, "ON", regions[0].Province)
	assert.Equal(t, 5, regions[0].Count)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCommunityRepository_SetSourceLink_Mock(t *testing.T) {
	repo, mock, cleanup := newMockCommunityRepo(t)
	defer cleanup()

	ctx := context.Background()

	mock.ExpectExec(regexp.QuoteMeta("UPDATE communities SET source_id")).
		WithArgs("c-1", "src-1", "https://example.com").
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := repo.SetSourceLink(ctx, "c-1", "src-1", "https://example.com")
	require.NoError(t, err)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCommunityRepository_SetSourceLink_NotFound(t *testing.T) {
	repo, mock, cleanup := newMockCommunityRepo(t)
	defer cleanup()

	ctx := context.Background()

	mock.ExpectExec(regexp.QuoteMeta("UPDATE communities SET source_id")).
		WithArgs("missing", "src-1", "https://example.com").
		WillReturnResult(sqlmock.NewResult(0, 0))

	err := repo.SetSourceLink(ctx, "missing", "src-1", "https://example.com")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCommunityRepository_BulkUpdateWebsite_Empty(t *testing.T) {
	repo, _, cleanup := newMockCommunityRepo(t)
	defer cleanup()

	ctx := context.Background()

	updated, err := repo.BulkUpdateWebsiteByInacID(ctx, []WebsiteUpdate{})
	require.NoError(t, err)
	assert.Equal(t, 0, updated)
}

func TestCommunityRepository_BulkUpdateWebsite_Mock(t *testing.T) {
	repo, mock, cleanup := newMockCommunityRepo(t)
	defer cleanup()

	ctx := context.Background()

	mock.ExpectExec(regexp.QuoteMeta("UPDATE communities SET website")).
		WithArgs("https://band1.ca", "100").
		WillReturnResult(sqlmock.NewResult(0, 1))

	mock.ExpectExec(regexp.QuoteMeta("UPDATE communities SET website")).
		WithArgs("https://band2.ca", "200").
		WillReturnResult(sqlmock.NewResult(0, 1))

	updated, err := repo.BulkUpdateWebsiteByInacID(ctx, []WebsiteUpdate{
		{InacID: "100", Website: "https://band1.ca"},
		{InacID: "200", Website: "https://band2.ca"},
	})
	require.NoError(t, err)
	assert.Equal(t, 2, updated)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCommunityRepository_UpsertByInacID_NilInacID(t *testing.T) {
	repo, _, cleanup := newMockCommunityRepo(t)
	defer cleanup()

	ctx := context.Background()
	c := &models.Community{Name: "Test", InacID: nil}

	err := repo.UpsertByInacID(ctx, c)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "inac_id is required")
}

func TestCommunityRepository_UpsertByStatCanCSD_NilCSD(t *testing.T) {
	repo, _, cleanup := newMockCommunityRepo(t)
	defer cleanup()

	ctx := context.Background()
	c := &models.Community{Name: "Test", StatCanCSD: nil}

	err := repo.UpsertByStatCanCSD(ctx, c)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "statcan_csd is required")
}

func TestBuildCommunityWhere_NoFilter(t *testing.T) {
	where, args := buildCommunityWhere(models.CommunityFilter{})
	assert.Empty(t, where)
	assert.Nil(t, args)
}

func TestBuildCommunityWhere_TypeFilter(t *testing.T) {
	where, args := buildCommunityWhere(models.CommunityFilter{Type: "city"})
	assert.Contains(t, where, "community_type = $1")
	require.Len(t, args, 1)
	assert.Equal(t, "city", args[0])
}

func TestBuildCommunityWhere_ProvinceFilter(t *testing.T) {
	where, args := buildCommunityWhere(models.CommunityFilter{Province: "ON"})
	assert.Contains(t, where, "province = $1")
	require.Len(t, args, 1)
}

func TestBuildCommunityWhere_SearchFilter(t *testing.T) {
	where, args := buildCommunityWhere(models.CommunityFilter{Search: "sudbury"})
	assert.Contains(t, where, "name ILIKE")
	require.Len(t, args, 1)
	assert.Equal(t, "%sudbury%", args[0])
}

func TestBuildCommunityWhere_Combined(t *testing.T) {
	where, args := buildCommunityWhere(models.CommunityFilter{
		Type:     "first_nation",
		Province: "ON",
		Search:   "wiki",
	})
	assert.Contains(t, where, "community_type")
	assert.Contains(t, where, "province")
	assert.Contains(t, where, "ILIKE")
	require.Len(t, args, 3)
}

func TestCommunityRepository_UpdateLastScrapedAt_Mock(t *testing.T) {
	repo, mock, cleanup := newMockCommunityRepo(t)
	defer cleanup()

	ctx := context.Background()
	now := time.Now()

	mock.ExpectExec(regexp.QuoteMeta("UPDATE communities SET last_scraped_at")).
		WithArgs("c-1", now).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := repo.UpdateLastScrapedAt(ctx, "c-1", now)
	require.NoError(t, err)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCommunityRepository_UpdateLastScrapedAt_NotFound(t *testing.T) {
	repo, mock, cleanup := newMockCommunityRepo(t)
	defer cleanup()

	ctx := context.Background()
	now := time.Now()

	mock.ExpectExec(regexp.QuoteMeta("UPDATE communities SET last_scraped_at")).
		WithArgs("missing", now).
		WillReturnResult(sqlmock.NewResult(0, 0))

	err := repo.UpdateLastScrapedAt(ctx, "missing", now)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCommunityRepository_Update_NotFound(t *testing.T) {
	repo, mock, cleanup := newMockCommunityRepo(t)
	defer cleanup()

	ctx := context.Background()
	c := &models.Community{ID: "missing", Name: "Test", Slug: "test"}

	mock.ExpectExec(regexp.QuoteMeta("UPDATE communities SET")).
		WillReturnResult(sqlmock.NewResult(0, 0))

	err := repo.Update(ctx, c)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCommunityRepository_Create_DBError(t *testing.T) {
	repo, mock, cleanup := newMockCommunityRepo(t)
	defer cleanup()

	ctx := context.Background()
	c := &models.Community{Name: "Fail", Slug: "fail", DataSource: "test"}

	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO communities")).
		WillReturnError(errors.New("duplicate key"))

	err := repo.Create(ctx, c)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create community")

	require.NoError(t, mock.ExpectationsWereMet())
}
