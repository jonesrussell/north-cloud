//nolint:testpackage // Testing unexported functions requires same-package access
package repository

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
	"github.com/jonesrussell/north-cloud/source-manager/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestLogger(t *testing.T) infralogger.Logger {
	t.Helper()
	log, err := infralogger.New(infralogger.Config{
		Level:       "debug",
		Format:      "json",
		Development: false,
	})
	require.NoError(t, err)
	return log
}

func newMockSourceRepo(t *testing.T) (*SourceRepository, sqlmock.Sqlmock, func()) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)

	repo := NewSourceRepository(db, newTestLogger(t))
	cleanup := func() {
		mock.ExpectClose()
		db.Close()
	}

	return repo, mock, cleanup
}

func TestSourceRepository_Create_Mock(t *testing.T) {
	repo, mock, cleanup := newMockSourceRepo(t)
	defer cleanup()

	ctx := context.Background()
	source := &models.Source{
		Name:      "Test Source",
		URL:       "https://example.com",
		RateLimit: "1s",
		MaxDepth:  2,
		Time:      models.StringArray{"09:00"},
		Selectors: models.SelectorConfig{
			Article: models.ArticleSelectors{Title: "h1"},
		},
		Enabled: true,
	}

	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO sources")).
		WithArgs(
			sqlmock.AnyArg(), // id
			source.Name, source.URL, source.RateLimit, source.MaxDepth,
			sqlmock.AnyArg(), // time JSON
			sqlmock.AnyArg(), // selectors JSON
			source.Enabled,
			sqlmock.AnyArg(), // feed_url
			sqlmock.AnyArg(), // sitemap_url
			sqlmock.AnyArg(), // ingestion_mode
			sqlmock.AnyArg(), // feed_poll_interval_minutes
			sqlmock.AnyArg(), // allow_source_discovery
			sqlmock.AnyArg(), // identity_key
			sqlmock.AnyArg(), // extraction_profile
			sqlmock.AnyArg(), // template_hint
			sqlmock.AnyArg(), // render_mode
			sqlmock.AnyArg(), // type
			sqlmock.AnyArg(), // indigenous_region
			sqlmock.AnyArg(), // created_at
			sqlmock.AnyArg(), // updated_at
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err := repo.Create(ctx, source)
	require.NoError(t, err)
	assert.NotEmpty(t, source.ID)
	assert.False(t, source.CreatedAt.IsZero())

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSourceRepository_GetByID_Mock(t *testing.T) {
	repo, mock, cleanup := newMockSourceRepo(t)
	defer cleanup()

	ctx := context.Background()
	now := time.Now()

	selectorsJSON := []byte(`{"article":{"title":"h1"},"list":{},"page":{}}`)
	timeJSON := []byte(`["09:00"]`)

	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, name, url")).
		WithArgs("test-id").
		WillReturnRows(
			sqlmock.NewRows([]string{
				"id", "name", "url", "rate_limit", "max_depth",
				"time", "selectors", "enabled",
				"feed_url", "sitemap_url", "ingestion_mode", "feed_poll_interval_minutes",
				"feed_disabled_at", "feed_disable_reason",
				"allow_source_discovery", "identity_key", "extraction_profile", "template_hint",
				"render_mode", "type", "indigenous_region",
				"disabled_at", "disable_reason",
				"created_at", "updated_at",
			}).AddRow(
				"test-id", "Test Source", "https://example.com", "1s", 2,
				timeJSON, selectorsJSON, true,
				nil, nil, "crawl", 0,
				nil, nil,
				false, nil, nil, nil,
				"static", "news", nil,
				nil, nil,
				now, now,
			),
		)

	source, err := repo.GetByID(ctx, "test-id")
	require.NoError(t, err)
	require.NotNil(t, source)
	assert.Equal(t, "test-id", source.ID)
	assert.Equal(t, "Test Source", source.Name)
	assert.Equal(t, "https://example.com", source.URL)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSourceRepository_GetByID_NotFound_Mock(t *testing.T) {
	repo, mock, cleanup := newMockSourceRepo(t)
	defer cleanup()

	ctx := context.Background()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, name, url")).
		WithArgs("not-found-id").
		WillReturnRows(sqlmock.NewRows(nil))

	source, err := repo.GetByID(ctx, "not-found-id")
	require.Error(t, err)
	assert.Nil(t, source)
	assert.Contains(t, err.Error(), "source not found")

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSourceRepository_Delete_Mock(t *testing.T) {
	repo, mock, cleanup := newMockSourceRepo(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		mock.ExpectExec(regexp.QuoteMeta("DELETE FROM sources WHERE id = $1")).
			WithArgs("del-id").
			WillReturnResult(sqlmock.NewResult(0, 1))

		err := repo.Delete(ctx, "del-id")
		require.NoError(t, err)
	})

	t.Run("not found", func(t *testing.T) {
		mock.ExpectExec(regexp.QuoteMeta("DELETE FROM sources WHERE id = $1")).
			WithArgs("missing-id").
			WillReturnResult(sqlmock.NewResult(0, 0))

		err := repo.Delete(ctx, "missing-id")
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrSourceNotFound)
	})

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSourceRepository_Update_NotFound_Mock(t *testing.T) {
	repo, mock, cleanup := newMockSourceRepo(t)
	defer cleanup()

	ctx := context.Background()
	source := &models.Source{
		ID:        "missing-id",
		Name:      "Test",
		URL:       "https://example.com",
		RateLimit: "1s",
		Selectors: models.SelectorConfig{},
		Time:      models.StringArray{"09:00"},
		Enabled:   true,
	}

	mock.ExpectExec(regexp.QuoteMeta("UPDATE sources")).
		WithArgs(
			source.ID, source.Name, source.URL, source.RateLimit, source.MaxDepth,
			sqlmock.AnyArg(), // time
			sqlmock.AnyArg(), // selectors
			source.Enabled,
			sqlmock.AnyArg(), // feed_url
			sqlmock.AnyArg(), // sitemap_url
			sqlmock.AnyArg(), // ingestion_mode
			sqlmock.AnyArg(), // feed_poll_interval_minutes
			sqlmock.AnyArg(), // allow_source_discovery
			sqlmock.AnyArg(), // identity_key
			sqlmock.AnyArg(), // extraction_profile
			sqlmock.AnyArg(), // template_hint
			sqlmock.AnyArg(), // render_mode
			sqlmock.AnyArg(), // type
			sqlmock.AnyArg(), // indigenous_region
			sqlmock.AnyArg(), // disable_reason
			sqlmock.AnyArg(), // updated_at
		).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT EXISTS(SELECT 1 FROM sources WHERE id = $1)")).
		WithArgs("missing-id").
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))

	err := repo.Update(ctx, source)
	require.ErrorIs(t, err, ErrSourceNotFound)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSourceRepository_DisableFeed_Mock(t *testing.T) {
	repo, mock, cleanup := newMockSourceRepo(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		mock.ExpectExec(regexp.QuoteMeta("UPDATE sources")).
			WithArgs("src-id", "not_found").
			WillReturnResult(sqlmock.NewResult(0, 1))

		err := repo.DisableFeed(ctx, "src-id", "not_found")
		require.NoError(t, err)
	})

	t.Run("not found", func(t *testing.T) {
		mock.ExpectExec(regexp.QuoteMeta("UPDATE sources")).
			WithArgs("missing-id", "gone").
			WillReturnResult(sqlmock.NewResult(0, 0))

		err := repo.DisableFeed(ctx, "missing-id", "gone")
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrSourceNotFound)
	})

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSourceRepository_EnableFeed_Mock(t *testing.T) {
	repo, mock, cleanup := newMockSourceRepo(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		mock.ExpectExec(regexp.QuoteMeta("UPDATE sources")).
			WithArgs("src-id").
			WillReturnResult(sqlmock.NewResult(0, 1))

		err := repo.EnableFeed(ctx, "src-id")
		require.NoError(t, err)
	})

	t.Run("not found", func(t *testing.T) {
		mock.ExpectExec(regexp.QuoteMeta("UPDATE sources")).
			WithArgs("missing-id").
			WillReturnResult(sqlmock.NewResult(0, 0))

		err := repo.EnableFeed(ctx, "missing-id")
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrSourceNotFound)
	})

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSourceRepository_DisableSource_Mock(t *testing.T) {
	repo, mock, cleanup := newMockSourceRepo(t)
	defer cleanup()

	ctx := context.Background()

	mock.ExpectExec(regexp.QuoteMeta("UPDATE sources")).
		WithArgs("src-id", "maintenance").
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := repo.DisableSource(ctx, "src-id", "maintenance")
	require.NoError(t, err)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSourceRepository_EnableSource_Mock(t *testing.T) {
	repo, mock, cleanup := newMockSourceRepo(t)
	defer cleanup()

	ctx := context.Background()

	mock.ExpectExec(regexp.QuoteMeta("UPDATE sources")).
		WithArgs("src-id").
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := repo.EnableSource(ctx, "src-id")
	require.NoError(t, err)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSourceRepository_GetByIdentityKey_Empty(t *testing.T) {
	repo, _, cleanup := newMockSourceRepo(t)
	defer cleanup()

	ctx := context.Background()

	source, err := repo.GetByIdentityKey(ctx, "")
	require.NoError(t, err)
	assert.Nil(t, source)
}

func TestSourceRepository_Count_Mock(t *testing.T) {
	repo, mock, cleanup := newMockSourceRepo(t)
	defer cleanup()

	ctx := context.Background()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM sources WHERE 1=1")).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(42))

	count, err := repo.Count(ctx, ListFilter{})
	require.NoError(t, err)
	assert.Equal(t, 42, count)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSourceRepository_GetCities_Mock(t *testing.T) {
	repo, mock, cleanup := newMockSourceRepo(t)
	defer cleanup()

	ctx := context.Background()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT name as source_name")).
		WillReturnRows(
			sqlmock.NewRows([]string{"source_name"}).
				AddRow("CBC News").
				AddRow("Global News"),
		)

	cities, err := repo.GetCities(ctx)
	require.NoError(t, err)
	require.Len(t, cities, 2)
	assert.Equal(t, "CBC News", cities[0].Name)
	assert.Contains(t, cities[0].Index, "cbc_news")
	assert.Equal(t, "Global News", cities[1].Name)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSourceRepository_UpsertSourcesTx_Empty(t *testing.T) {
	repo, _, cleanup := newMockSourceRepo(t)
	defer cleanup()

	ctx := context.Background()

	created, updated, err := repo.UpsertSourcesTx(ctx, []*models.Source{})
	require.NoError(t, err)
	assert.Nil(t, created)
	assert.Nil(t, updated)
}
