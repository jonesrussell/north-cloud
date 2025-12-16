package repository

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/jonesrussell/gosources/internal/models"
	"github.com/jonesrussell/gosources/internal/testhelpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestDB creates a test database for integration tests
// This requires a local PostgreSQL instance or can be adapted for testcontainers
// Set GOSOURCES_TEST_DB environment variable to customize connection
func setupTestDB(t *testing.T) (*sql.DB, func()) {
	// Skip if running in short mode (unit tests only)
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Try to connect to a test database
	// In CI/CD or with testcontainers, this would be set up differently
	connStr := "host=localhost port=5432 user=postgres password=postgres dbname=gosources_test sslmode=disable"
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		t.Skipf("Skipping test: could not connect to test database: %v", err)
	}

	ctx := context.Background()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		t.Skipf("Skipping test: could not ping test database: %v", err)
	}

	// Run migrations
	logger := testhelpers.NewTestLogger()
	if err := testhelpers.RunMigrations(ctx, db, logger); err != nil {
		db.Close()
		t.Skipf("Skipping test: could not run migrations: %v", err)
	}

	// Clean up function
	cleanup := func() {
		// Clean up test data
		ctx := context.Background()
		_, _ = db.ExecContext(ctx, "TRUNCATE TABLE sources CASCADE")
		db.Close()
	}

	return db, cleanup
}

func TestSourceRepository_Create(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	logger := testhelpers.NewTestLogger()
	repo := NewSourceRepository(db, logger)
	ctx := context.Background()

	tests := []struct {
		name    string
		source  *models.Source
		wantErr bool
	}{
		{
			name: "create valid source",
			source: &models.Source{
				Name:         "Test Source",
				URL:          "https://example.com",
				ArticleIndex: "articles",
				PageIndex:    "pages",
				RateLimit:    "1s",
				MaxDepth:     2,
				Time:         models.StringArray{"09:00", "17:00"},
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
				Name:         "City Source",
				URL:          "https://city.com",
				ArticleIndex: "city_articles",
				PageIndex:    "city_pages",
				RateLimit:    "2s",
				MaxDepth:     3,
				Time:         models.StringArray{"08:00"},
				Selectors: models.SelectorConfig{
					Article: models.ArticleSelectors{
						Title: "article h1",
					},
				},
				CityName: stringPtr("TestCity"),
				GroupID:  stringPtr("group-uuid-123"),
				Enabled:  true,
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
	repo := NewSourceRepository(db, logger)
	ctx := context.Background()

	// Create a test source
	testSource := &models.Source{
		Name:         "Get Test Source",
		URL:          "https://gettest.com",
		ArticleIndex: "articles",
		PageIndex:    "pages",
		RateLimit:    "1s",
		MaxDepth:     2,
		Time:         models.StringArray{"10:00"},
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
			source, err := repo.GetByID(ctx, tt.id)
			if (err != nil) != tt.wantErr {
				t.Errorf("SourceRepository.GetByID() error = %v, wantErr %v", err, tt.wantErr)
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
	repo := NewSourceRepository(db, logger)
	ctx := context.Background()

	// Create multiple test sources
	sources := []*models.Source{
		{
			Name:         "Source A",
			URL:          "https://sourcea.com",
			ArticleIndex: "articles",
			PageIndex:    "pages",
			RateLimit:    "1s",
			MaxDepth:     2,
			Time:         models.StringArray{"09:00"},
			Selectors: models.SelectorConfig{
				Article: models.ArticleSelectors{Title: "h1"},
			},
			Enabled: true,
		},
		{
			Name:         "Source B",
			URL:          "https://sourceb.com",
			ArticleIndex: "articles",
			PageIndex:    "pages",
			RateLimit:    "1s",
			MaxDepth:     2,
			Time:         models.StringArray{"10:00"},
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
	repo := NewSourceRepository(db, logger)
	ctx := context.Background()

	// Create a test source
	testSource := &models.Source{
		Name:         "Update Test Source",
		URL:          "https://updatetest.com",
		ArticleIndex: "articles",
		PageIndex:    "pages",
		RateLimit:    "1s",
		MaxDepth:     2,
		Time:         models.StringArray{"09:00"},
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
	repo := NewSourceRepository(db, logger)
	ctx := context.Background()

	// Create a test source
	testSource := &models.Source{
		Name:         "Delete Test Source",
		URL:          "https://deletetest.com",
		ArticleIndex: "articles",
		PageIndex:    "pages",
		RateLimit:    "1s",
		MaxDepth:     2,
		Time:         models.StringArray{"09:00"},
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
	assert.Error(t, err, "GetByID() should error for deleted source")

	// Try to delete non-existent source
	err = repo.Delete(ctx, "non-existent-id")
	assert.Error(t, err, "Delete() should error for non-existent source")
}

func TestSourceRepository_GetCities(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	logger := testhelpers.NewTestLogger()
	repo := NewSourceRepository(db, logger)
	ctx := context.Background()

	// Create enabled sources with city names
	citySources := []*models.Source{
		{
			Name:         "City A Source",
			URL:          "https://citya.com",
			ArticleIndex: "city_a_articles",
			PageIndex:    "city_a_pages",
			RateLimit:    "1s",
			MaxDepth:     2,
			Time:         models.StringArray{"09:00"},
			Selectors: models.SelectorConfig{
				Article: models.ArticleSelectors{Title: "h1"},
			},
			CityName: stringPtr("CityA"),
			GroupID:  stringPtr("group-a"),
			Enabled:  true,
		},
		{
			Name:         "City B Source",
			URL:          "https://cityb.com",
			ArticleIndex: "city_b_articles",
			PageIndex:    "city_b_pages",
			RateLimit:    "1s",
			MaxDepth:     2,
			Time:         models.StringArray{"10:00"},
			Selectors: models.SelectorConfig{
				Article: models.ArticleSelectors{Title: "h1"},
			},
			CityName: stringPtr("CityB"),
			Enabled:  true,
		},
		{
			Name:         "Disabled Source",
			URL:          "https://disabled.com",
			ArticleIndex: "disabled_articles",
			PageIndex:    "disabled_pages",
			RateLimit:    "1s",
			MaxDepth:     2,
			Time:         models.StringArray{"11:00"},
			Selectors: models.SelectorConfig{
				Article: models.ArticleSelectors{Title: "h1"},
			},
			CityName: stringPtr("DisabledCity"),
			Enabled:  false, // Disabled
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
	assert.True(t, cityNames["CityA"], "CityA should be in results")
	assert.True(t, cityNames["CityB"], "CityB should be in results")
	assert.False(t, cityNames["DisabledCity"], "DisabledCity should not be in results")
}

// Helper function
func stringPtr(s string) *string {
	return &s
}

