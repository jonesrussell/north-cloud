//nolint:testpackage // Testing with sqlmock requires same-package access for helper
package repository

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jonesrussell/north-cloud/source-manager/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sourceListColumns() []string {
	return []string{
		"id", "name", "url", "rate_limit", "max_depth",
		"time", "selectors", "enabled",
		"feed_url", "sitemap_url", "ingestion_mode", "feed_poll_interval_minutes",
		"feed_disabled_at", "feed_disable_reason",
		"allow_source_discovery", "identity_key", "extraction_profile", "template_hint",
		"render_mode", "type", "indigenous_region",
		"disabled_at", "disable_reason",
		"created_at", "updated_at",
	}
}

func addSourceRow(rows *sqlmock.Rows, id, name string) {
	now := time.Now()
	selectorsJSON := []byte(`{"article":{"title":"h1"},"list":{},"page":{}}`)
	timeJSON := []byte(`["09:00"]`)

	rows.AddRow(
		id, name, "https://example.com/"+id, "1s", 2,
		timeJSON, selectorsJSON, true,
		nil, nil, "crawl", 0,
		nil, nil,
		false, nil, nil, nil,
		"static", "news", nil,
		nil, nil,
		now, now,
	)
}

func TestSourceRepository_List_Mock(t *testing.T) {
	repo, mock, cleanup := newMockSourceRepo(t)
	defer cleanup()

	ctx := context.Background()

	rows := sqlmock.NewRows(sourceListColumns())
	addSourceRow(rows, "id-1", "Alpha Source")
	addSourceRow(rows, "id-2", "Beta Source")

	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, name, url")).
		WillReturnRows(rows)

	sources, err := repo.List(ctx)
	require.NoError(t, err)
	require.Len(t, sources, 2)
	assert.Equal(t, "Alpha Source", sources[0].Name)
	assert.Equal(t, "Beta Source", sources[1].Name)
	// Verify MergeWithDefaults was applied
	assert.NotEmpty(t, sources[0].Selectors.Article.Container)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSourceRepository_List_EmptyResult(t *testing.T) {
	repo, mock, cleanup := newMockSourceRepo(t)
	defer cleanup()

	ctx := context.Background()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, name, url")).
		WillReturnRows(sqlmock.NewRows(sourceListColumns()))

	sources, err := repo.List(ctx)
	require.NoError(t, err)
	assert.Empty(t, sources)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSourceRepository_ListPaginated_Mock(t *testing.T) {
	repo, mock, cleanup := newMockSourceRepo(t)
	defer cleanup()

	ctx := context.Background()

	rows := sqlmock.NewRows(sourceListColumns())
	addSourceRow(rows, "id-1", "Source 1")

	mock.ExpectQuery("SELECT id, name, url").
		WithArgs(10, 0).
		WillReturnRows(rows)

	sources, err := repo.ListPaginated(ctx, ListFilter{
		Limit:  10,
		Offset: 0,
	})
	require.NoError(t, err)
	require.Len(t, sources, 1)
	assert.Equal(t, "Source 1", sources[0].Name)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSourceRepository_ListPaginated_WithSearch(t *testing.T) {
	repo, mock, cleanup := newMockSourceRepo(t)
	defer cleanup()

	ctx := context.Background()

	rows := sqlmock.NewRows(sourceListColumns())
	addSourceRow(rows, "id-1", "CBC News")

	mock.ExpectQuery("SELECT id, name, url").
		WithArgs("%cbc%", 20, 0).
		WillReturnRows(rows)

	sources, err := repo.ListPaginated(ctx, ListFilter{
		Limit:  20,
		Offset: 0,
		Search: "cbc",
	})
	require.NoError(t, err)
	require.Len(t, sources, 1)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSourceRepository_ListPaginated_WithEnabledFilter(t *testing.T) {
	repo, mock, cleanup := newMockSourceRepo(t)
	defer cleanup()

	ctx := context.Background()

	rows := sqlmock.NewRows(sourceListColumns())
	addSourceRow(rows, "id-1", "Enabled Source")

	enabled := true
	mock.ExpectQuery("SELECT id, name, url").
		WithArgs(true, 100, 0).
		WillReturnRows(rows)

	sources, err := repo.ListPaginated(ctx, ListFilter{
		Limit:   100,
		Offset:  0,
		Enabled: &enabled,
	})
	require.NoError(t, err)
	require.Len(t, sources, 1)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSourceRepository_GetByIdentityKey_Found(t *testing.T) {
	repo, mock, cleanup := newMockSourceRepo(t)
	defer cleanup()

	ctx := context.Background()

	rows := sqlmock.NewRows(sourceListColumns())
	addSourceRow(rows, "id-1", "Found Source")

	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, name, url")).
		WithArgs("example.com/news").
		WillReturnRows(rows)

	source, err := repo.GetByIdentityKey(ctx, "example.com/news")
	require.NoError(t, err)
	require.NotNil(t, source)
	assert.Equal(t, "Found Source", source.Name)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSourceRepository_GetByIdentityKey_NotFound(t *testing.T) {
	repo, mock, cleanup := newMockSourceRepo(t)
	defer cleanup()

	ctx := context.Background()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, name, url")).
		WithArgs("no-match").
		WillReturnRows(sqlmock.NewRows(sourceListColumns()))

	source, err := repo.GetByIdentityKey(ctx, "no-match")
	require.NoError(t, err)
	assert.Nil(t, source)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSourceRepository_Update_Success_Mock(t *testing.T) {
	repo, mock, cleanup := newMockSourceRepo(t)
	defer cleanup()

	ctx := context.Background()
	source := &models.Source{
		ID:        "upd-id",
		Name:      "Updated",
		URL:       "https://updated.com",
		RateLimit: "5s",
		MaxDepth:  3,
		Selectors: models.SelectorConfig{},
		Time:      models.StringArray{"09:00"},
		Enabled:   true,
	}

	mock.ExpectExec(regexp.QuoteMeta("UPDATE sources")).
		WithArgs(
			source.ID, source.Name, source.URL, source.RateLimit, source.MaxDepth,
			sqlmock.AnyArg(), sqlmock.AnyArg(), // time, selectors
			source.Enabled,
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), // disable_reason
			sqlmock.AnyArg(), // updated_at
		).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := repo.Update(ctx, source)
	require.NoError(t, err)

	require.NoError(t, mock.ExpectationsWereMet())
}
