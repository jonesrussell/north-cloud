package repository_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/jonesrussell/north-cloud/source-manager/internal/models"
	"github.com/jonesrussell/north-cloud/source-manager/internal/repository"
	"github.com/jonesrussell/north-cloud/source-manager/internal/testhelpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestDB creates a test database for integration tests
// This requires a local PostgreSQL instance or can be adapted for testcontainers
// Set GOSOURCES_TEST_DB environment variable to customize connection
func setupTestDB(t *testing.T) (db *sql.DB, cleanup func()) {
	t.Helper()
	// Skip if running in short mode (unit tests only)
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Try to connect to a test database
	// In CI/CD or with testcontainers, this would be set up differently
	connStr := "host=localhost port=5432 user=postgres password=postgres dbname=source_manager_test sslmode=disable"
	var err error
	db, err = sql.Open("postgres", connStr)
	if err != nil {
		t.Skipf("Skipping test: could not connect to test database: %v", err)
	}

	testCtx := context.Background()
	if pingErr := db.PingContext(testCtx); pingErr != nil {
		db.Close()
		t.Skipf("Skipping test: could not ping test database: %v", pingErr)
	}

	// Run migrations
	logger := testhelpers.NewTestLogger()
	if migrateErr := testhelpers.RunMigrations(testCtx, db, logger); migrateErr != nil {
		db.Close()
		t.Skipf("Skipping test: could not run migrations: %v", migrateErr)
	}

	// Clean up function
	cleanup = func() {
		// Clean up test data
		cleanupCtx := context.Background()
		_, _ = db.ExecContext(cleanupCtx, "TRUNCATE TABLE sources CASCADE")
		db.Close()
	}

	return db, cleanup
}

func TestSourceRepository_Create(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	logger := testhelpers.NewTestLogger()
	repo := repository.NewSourceRepository(db, logger)
	ctx := context.Background()

	tests := []struct {
		name    string
		source  *models.Source
		wantErr bool
	}{
		{
			name: "create valid source",
			source: &models.Source{
				Name:      "Test Source",
				URL:       "https://example.com",
				RateLimit: "1s",
				MaxDepth:  2,
				Time:      models.StringArray{"09:00", "17:00"},
				Selectors: models.SelectorConfig{
					Article: models.ArticleSelectors{
						Title: "h1",
						Body:  ".content",
					},
				},
				Enabled: true,
			},
			wantErr: false,
		},
		{
			name: "create source with city mapping",
			source: &models.Source{
				Name:      "City Source",
				URL:       "https://city.com",
				RateLimit: "2s",
				MaxDepth:  3,
				Time:      models.StringArray{"08:00"},
				Selectors: models.SelectorConfig{
					Article: models.ArticleSelectors{
						Title: "article h1",
					},
				},
				Enabled: true,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := repo.Create(ctx, tt.source)
			if (err != nil) != tt.wantErr {
				t.Errorf("SourceRepository.Create() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				// Verify source was created
				assert.NotEmpty(t, tt.source.ID, "Source ID should be set")
				assert.False(t, tt.source.CreatedAt.IsZero(), "CreatedAt should be set")
				assert.False(t, tt.source.UpdatedAt.IsZero(), "UpdatedAt should be set")
			}
		})
	}
}

func TestSourceRepository_GetByID(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	logger := testhelpers.NewTestLogger()
	repo := repository.NewSourceRepository(db, logger)
	ctx := context.Background()

	// Create a test source
	testSource := &models.Source{
		Name:      "Get Test Source",
		URL:       "https://gettest.com",
		RateLimit: "1s",
		MaxDepth:  2,
		Time:      models.StringArray{"10:00"},
		Selectors: models.SelectorConfig{
			Article: models.ArticleSelectors{
				Title: "h1",
			},
		},
		Enabled: true,
	}

	err := repo.Create(ctx, testSource)
	require.NoError(t, err, "Failed to create test source")

	tests := []struct {
		name    string
		id      string
		wantErr bool
	}{
		{
			name:    "get existing source",
			id:      testSource.ID,
			wantErr: false,
		},
		{
			name:    "get non-existent source",
			id:      "non-existent-id",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source, getErr := repo.GetByID(ctx, tt.id)
			if (getErr != nil) != tt.wantErr {
				t.Errorf("SourceRepository.GetByID() error = %v, wantErr %v", getErr, tt.wantErr)
				return
			}
			if !tt.wantErr {
				require.NotNil(t, source, "Source should not be nil")
				assert.Equal(t, tt.id, source.ID)
				assert.Equal(t, testSource.Name, source.Name)
				assert.Equal(t, testSource.URL, source.URL)
			}
		})
	}
}

func TestSourceRepository_List(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	logger := testhelpers.NewTestLogger()
	repo := repository.NewSourceRepository(db, logger)
	ctx := context.Background()

	// Create multiple test sources
	sources := []*models.Source{
		{
			Name:      "Source A",
			URL:       "https://sourcea.com",
			RateLimit: "1s",
			MaxDepth:  2,
			Time:      models.StringArray{"09:00"},
			Selectors: models.SelectorConfig{
				Article: models.ArticleSelectors{Title: "h1"},
			},
			Enabled: true,
		},
		{
			Name:      "Source B",
			URL:       "https://sourceb.com",
			RateLimit: "1s",
			MaxDepth:  2,
			Time:      models.StringArray{"10:00"},
			Selectors: models.SelectorConfig{
				Article: models.ArticleSelectors{Title: "h1"},
			},
			Enabled: true,
		},
	}

	for _, s := range sources {
		err := repo.Create(ctx, s)
		require.NoError(t, err, "Failed to create test source")
	}

	list, err := repo.List(ctx)
	require.NoError(t, err, "List() should not error")
	assert.GreaterOrEqual(t, len(list), len(sources), "List should return at least the created sources")

	// Verify sources are ordered by name
	if len(list) >= 2 {
		for i := 1; i < len(list); i++ {
			assert.LessOrEqual(t, list[i-1].Name, list[i].Name, "Sources should be sorted by name")
		}
	}
}

func TestSourceRepository_Update(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	logger := testhelpers.NewTestLogger()
	repo := repository.NewSourceRepository(db, logger)
	ctx := context.Background()

	// Create a test source
	testSource := &models.Source{
		Name:      "Update Test Source",
		URL:       "https://updatetest.com",
		RateLimit: "1s",
		MaxDepth:  2,
		Time:      models.StringArray{"09:00"},
		Selectors: models.SelectorConfig{
			Article: models.ArticleSelectors{Title: "h1"},
		},
		Enabled: true,
	}

	err := repo.Create(ctx, testSource)
	require.NoError(t, err, "Failed to create test source")

	originalUpdatedAt := testSource.UpdatedAt

	// Wait a bit to ensure updated_at changes
	time.Sleep(10 * time.Millisecond)

	// Update the source
	testSource.Name = "Updated Source Name"
	testSource.MaxDepth = 5
	err = repo.Update(ctx, testSource)
	require.NoError(t, err, "Update() should not error")

	// Verify update
	updated, err := repo.GetByID(ctx, testSource.ID)
	require.NoError(t, err, "GetByID() should not error")
	assert.Equal(t, "Updated Source Name", updated.Name)
	assert.Equal(t, 5, updated.MaxDepth)
	assert.True(t, updated.UpdatedAt.After(originalUpdatedAt), "UpdatedAt should be updated")
}

func TestSourceRepository_Delete(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	logger := testhelpers.NewTestLogger()
	repo := repository.NewSourceRepository(db, logger)
	ctx := context.Background()

	// Create a test source
	testSource := &models.Source{
		Name:      "Delete Test Source",
		URL:       "https://deletetest.com",
		RateLimit: "1s",
		MaxDepth:  2,
		Time:      models.StringArray{"09:00"},
		Selectors: models.SelectorConfig{
			Article: models.ArticleSelectors{Title: "h1"},
		},
		Enabled: true,
	}

	err := repo.Create(ctx, testSource)
	require.NoError(t, err, "Failed to create test source")

	// Delete the source
	err = repo.Delete(ctx, testSource.ID)
	require.NoError(t, err, "Delete() should not error")

	// Verify deletion
	_, err = repo.GetByID(ctx, testSource.ID)
	require.Error(t, err, "GetByID() should error for deleted source")

	// Try to delete non-existent source
	err = repo.Delete(ctx, "non-existent-id")
	assert.Error(t, err, "Delete() should error for non-existent source")
}

func TestSourceRepository_GetCities(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	logger := testhelpers.NewTestLogger()
	repo := repository.NewSourceRepository(db, logger)
	ctx := context.Background()

	// Create enabled sources with city names
	citySources := []*models.Source{
		{
			Name:      "City A Source",
			URL:       "https://citya.com",
			RateLimit: "1s",
			MaxDepth:  2,
			Time:      models.StringArray{"09:00"},
			Selectors: models.SelectorConfig{
				Article: models.ArticleSelectors{Title: "h1"},
			},
			Enabled: true,
		},
		{
			Name:      "City B Source",
			URL:       "https://cityb.com",
			RateLimit: "1s",
			MaxDepth:  2,
			Time:      models.StringArray{"10:00"},
			Selectors: models.SelectorConfig{
				Article: models.ArticleSelectors{Title: "h1"},
			},
			Enabled: true,
		},
		{
			Name:      "Disabled Source",
			URL:       "https://disabled.com",
			RateLimit: "1s",
			MaxDepth:  2,
			Time:      models.StringArray{"11:00"},
			Selectors: models.SelectorConfig{
				Article: models.ArticleSelectors{Title: "h1"},
			},
			Enabled: false, // Disabled
		},
	}

	for _, s := range citySources {
		err := repo.Create(ctx, s)
		require.NoError(t, err, "Failed to create test source")
	}

	cities, err := repo.GetCities(ctx)
	require.NoError(t, err, "GetCities() should not error")

	// Should only return enabled cities
	assert.Len(t, cities, 2, "Should return 2 enabled cities")
	cityNames := make(map[string]bool)
	for _, city := range cities {
		cityNames[city.Name] = true
		assert.NotEmpty(t, city.Index, "City index should not be empty")
	}
	assert.True(t, cityNames["City A Source"], "City A Source should be in results")
	assert.True(t, cityNames["City B Source"], "City B Source should be in results")
	assert.False(t, cityNames["Disabled Source"], "Disabled Source should not be in results")
}

func TestSourceRepository_UpsertSourcesTx(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	logger := testhelpers.NewTestLogger()
	repo := repository.NewSourceRepository(db, logger)
	ctx := context.Background()

	t.Run("batch insert all new", func(t *testing.T) {
		sources := []*models.Source{
			{
				Name:      "Batch New Source 1",
				URL:       "https://batch-new-1.com",
				RateLimit: "1s",
				MaxDepth:  2,
				Time:      models.StringArray{"09:00"},
				Selectors: models.SelectorConfig{
					Article: models.ArticleSelectors{Title: "h1", Body: ".content"},
				},
				Enabled: true,
			},
			{
				Name:      "Batch New Source 2",
				URL:       "https://batch-new-2.com",
				RateLimit: "2s",
				MaxDepth:  3,
				Time:      models.StringArray{"10:00"},
				Selectors: models.SelectorConfig{
					Article: models.ArticleSelectors{Title: "h2", Body: ".body"},
				},
				Enabled: true,
			},
		}

		created, updated, err := repo.UpsertSourcesTx(ctx, sources)
		require.NoError(t, err)
		assert.Equal(t, 2, created, "Should create 2 new sources")
		assert.Equal(t, 0, updated, "Should update 0 sources")

		// Verify sources were persisted
		for _, s := range sources {
			assert.NotEmpty(t, s.ID, "Source ID should be set")
			fetched, fetchErr := repo.GetByID(ctx, s.ID)
			require.NoError(t, fetchErr)
			assert.Equal(t, s.Name, fetched.Name)
		}
	})

	t.Run("batch with mix of new and existing", func(t *testing.T) {
		// First create an existing source
		existingSource := &models.Source{
			Name:      "Batch Existing Source",
			URL:       "https://batch-existing.com",
			RateLimit: "1s",
			MaxDepth:  2,
			Time:      models.StringArray{"09:00"},
			Selectors: models.SelectorConfig{
				Article: models.ArticleSelectors{Title: "h1", Body: ".content"},
			},
			Enabled: true,
		}
		err := repo.Create(ctx, existingSource)
		require.NoError(t, err)
		originalID := existingSource.ID

		// Now upsert with the existing source (updated) and a new source
		sources := []*models.Source{
			{
				Name:      "Batch Existing Source", // Same name - will be updated
				URL:       "https://batch-existing-updated.com",
				RateLimit: "5s",
				MaxDepth:  10,
				Time:      models.StringArray{"12:00"},
				Selectors: models.SelectorConfig{
					Article: models.ArticleSelectors{Title: "h1.updated", Body: ".updated"},
				},
				Enabled: false,
			},
			{
				Name:      "Batch Mix New Source",
				URL:       "https://batch-mix-new.com",
				RateLimit: "3s",
				MaxDepth:  4,
				Time:      models.StringArray{"14:00"},
				Selectors: models.SelectorConfig{
					Article: models.ArticleSelectors{Title: "h3", Body: ".new-body"},
				},
				Enabled: true,
			},
		}

		created, updated, err := repo.UpsertSourcesTx(ctx, sources)
		require.NoError(t, err)
		assert.Equal(t, 1, created, "Should create 1 new source")
		assert.Equal(t, 1, updated, "Should update 1 source")

		// Verify the existing source was updated with the original ID
		assert.Equal(t, originalID, sources[0].ID, "Updated source should keep original ID")
		fetched, fetchErr := repo.GetByID(ctx, originalID)
		require.NoError(t, fetchErr)
		assert.Equal(t, "https://batch-existing-updated.com", fetched.URL)
		assert.Equal(t, "5s", fetched.RateLimit)
		assert.False(t, fetched.Enabled)

		// Verify the new source was created
		assert.NotEmpty(t, sources[1].ID, "New source should have ID set")
		newFetched, newErr := repo.GetByID(ctx, sources[1].ID)
		require.NoError(t, newErr)
		assert.Equal(t, "Batch Mix New Source", newFetched.Name)
	})

	t.Run("empty batch", func(t *testing.T) {
		sources := []*models.Source{}

		created, updated, err := repo.UpsertSourcesTx(ctx, sources)
		require.NoError(t, err)
		assert.Equal(t, 0, created, "Should create 0 sources")
		assert.Equal(t, 0, updated, "Should update 0 sources")
	})
}

func TestSourceRepository_UpsertSource(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	logger := testhelpers.NewTestLogger()
	repo := repository.NewSourceRepository(db, logger)
	ctx := context.Background()

	t.Run("insert new source", func(t *testing.T) {
		// Start a transaction
		tx, err := db.BeginTx(ctx, nil)
		require.NoError(t, err)

		source := &models.Source{
			Name:      "Upsert New Source",
			URL:       "https://upsert-new.com",
			RateLimit: "2s",
			MaxDepth:  3,
			Time:      models.StringArray{"10:00", "14:00"},
			Selectors: models.SelectorConfig{
				Article: models.ArticleSelectors{
					Title: "h1.title",
					Body:  ".article-body",
				},
			},
			Enabled: true,
		}

		created, err := repo.UpsertSource(ctx, tx, source)
		require.NoError(t, err)
		require.NoError(t, tx.Commit())

		// Should be a new insert
		assert.True(t, created, "UpsertSource should return true for new source")
		assert.NotEmpty(t, source.ID, "Source ID should be set")
		assert.False(t, source.CreatedAt.IsZero(), "CreatedAt should be set")
		assert.False(t, source.UpdatedAt.IsZero(), "UpdatedAt should be set")

		// Verify source was persisted
		fetched, err := repo.GetByID(ctx, source.ID)
		require.NoError(t, err)
		assert.Equal(t, "Upsert New Source", fetched.Name)
		assert.Equal(t, "https://upsert-new.com", fetched.URL)
		assert.Equal(t, "2s", fetched.RateLimit)
		assert.Equal(t, 3, fetched.MaxDepth)
		assert.True(t, fetched.Enabled)
	})

	t.Run("update existing source", func(t *testing.T) {
		// First, create a source normally
		existingSource := &models.Source{
			Name:      "Upsert Existing Source",
			URL:       "https://original-url.com",
			RateLimit: "1s",
			MaxDepth:  2,
			Time:      models.StringArray{"09:00"},
			Selectors: models.SelectorConfig{
				Article: models.ArticleSelectors{
					Title: "h1",
					Body:  ".content",
				},
			},
			Enabled: true,
		}

		err := repo.Create(ctx, existingSource)
		require.NoError(t, err)
		originalID := existingSource.ID

		// Now upsert with same name but different data
		tx, err := db.BeginTx(ctx, nil)
		require.NoError(t, err)

		updatedSource := &models.Source{
			Name:      "Upsert Existing Source", // Same name
			URL:       "https://updated-url.com",
			RateLimit: "5s",
			MaxDepth:  10,
			Time:      models.StringArray{"12:00", "18:00"},
			Selectors: models.SelectorConfig{
				Article: models.ArticleSelectors{
					Title: "h2.updated-title",
					Body:  ".updated-body",
				},
			},
			Enabled: false,
		}

		created, err := repo.UpsertSource(ctx, tx, updatedSource)
		require.NoError(t, err)
		require.NoError(t, tx.Commit())

		// Should be an update, not insert
		assert.False(t, created, "UpsertSource should return false for existing source")
		assert.Equal(t, originalID, updatedSource.ID, "ID should match the original source")

		// Verify source was updated
		fetched, err := repo.GetByID(ctx, originalID)
		require.NoError(t, err)
		assert.Equal(t, "Upsert Existing Source", fetched.Name)
		assert.Equal(t, "https://updated-url.com", fetched.URL)
		assert.Equal(t, "5s", fetched.RateLimit)
		assert.Equal(t, 10, fetched.MaxDepth)
		assert.False(t, fetched.Enabled)
	})
}
