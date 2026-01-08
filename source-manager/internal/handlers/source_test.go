package handlers_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jonesrussell/north-cloud/source-manager/internal/handlers"
	"github.com/jonesrussell/north-cloud/source-manager/internal/models"
	"github.com/jonesrussell/north-cloud/source-manager/internal/repository"
	"github.com/jonesrussell/north-cloud/source-manager/internal/testhelpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// NOTE: Handler tests currently have limitations because handlers use concrete repository types.
// For proper unit testing, handlers should be refactored to accept a repository interface.
// These tests demonstrate the test structure but may require a real database connection
// or handler refactoring to work properly.

// MockSourceRepository is a mock implementation of repository methods
type MockSourceRepository struct {
	mock.Mock
}

func (m *MockSourceRepository) Create(ctx context.Context, source *models.Source) error {
	args := m.Called(ctx, source)
	if args.Get(0) == nil {
		return nil
	}
	return args.Error(0)
}

func (m *MockSourceRepository) GetByID(ctx context.Context, id string) (*models.Source, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Source), args.Error(1)
}

func (m *MockSourceRepository) List(ctx context.Context) ([]models.Source, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.Source), args.Error(1)
}

func (m *MockSourceRepository) Update(ctx context.Context, source *models.Source) error {
	args := m.Called(ctx, source)
	if args.Get(0) == nil {
		return nil
	}
	return args.Error(0)
}

func (m *MockSourceRepository) Delete(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil
	}
	return args.Error(0)
}

func (m *MockSourceRepository) GetCities(ctx context.Context) ([]models.City, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.City), args.Error(1)
}

func setupRouter(handler *handlers.SourceHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/sources", handler.Create)
	router.GET("/sources", handler.List)
	router.GET("/sources/:id", handler.GetByID)
	router.PUT("/sources/:id", handler.Update)
	router.DELETE("/sources/:id", handler.Delete)
	router.GET("/cities", handler.GetCities)
	return router
}

// TestSourceHandler_Create is skipped until handlers use repository interfaces
// TODO: Refactor handlers to accept repository interface for proper mocking
func TestSourceHandler_Create(t *testing.T) {
	t.Skip("Skipping handler tests until handlers use repository interfaces")
	tests := []struct {
		name           string
		requestBody    any
		mockSetup      func(*MockSourceRepository)
		expectedStatus int
		validateResp   func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name: "create valid source",
			requestBody: models.Source{
				Name:      "Test Source",
				URL:       "https://example.com",
				RateLimit: "1s",
				MaxDepth:  2,
				Time:      models.StringArray{"09:00"},
				Selectors: models.SelectorConfig{
					Article: models.ArticleSelectors{Title: "h1"},
				},
				Enabled: true,
			},
			mockSetup: func(m *MockSourceRepository) {
				m.On("Create", mock.Anything, mock.MatchedBy(func(s *models.Source) bool {
					return s.Name == "Test Source" && s.ID != ""
				})).Return(nil)
			},
			expectedStatus: http.StatusCreated,
			validateResp: func(t *testing.T, w *httptest.ResponseRecorder) {
				var source models.Source
				err := json.Unmarshal(w.Body.Bytes(), &source)
				require.NoError(t, err)
				assert.Equal(t, "Test Source", source.Name)
				assert.NotEmpty(t, source.ID)
			},
		},
		{
			name: "invalid request body",
			requestBody: map[string]string{
				"invalid": "json",
			},
			mockSetup:      func(m *MockSourceRepository) {},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "repository error",
			requestBody: models.Source{
				Name:      "Test Source",
				URL:       "https://example.com",
				RateLimit: "1s",
				MaxDepth:  2,
				Time:      models.StringArray{"09:00"},
				Selectors: models.SelectorConfig{
					Article: models.ArticleSelectors{Title: "h1"},
				},
			},
			mockSetup: func(m *MockSourceRepository) {
				m.On("Create", mock.Anything, mock.Anything).Return(errors.New("database error"))
			},
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := new(MockSourceRepository)
			tt.mockSetup(mockRepo)

			logger := testhelpers.NewTestLogger()
			// TODO: Handler needs interface injection for proper mocking
			// For now, create a dummy repository (tests may require actual DB)
			db, _ := sql.Open("postgres", "host=localhost port=5432 user=postgres password=postgres dbname=gosources_test sslmode=disable")
			repo := repository.NewSourceRepository(db, logger)
			handler := handlers.NewSourceHandler(repo, logger)

			router := setupRouter(handler)

			body, _ := json.Marshal(tt.requestBody)
			req := httptest.NewRequest(http.MethodPost, "/sources", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.validateResp != nil {
				tt.validateResp(t, w)
			}
			mockRepo.AssertExpectations(t)
		})
	}
}

func TestSourceHandler_GetByID(t *testing.T) {
	t.Skip("Skipping handler tests until handlers use repository interfaces")
	testID := uuid.New().String()
	testSource := &models.Source{
		ID:        testID,
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

	tests := []struct {
		name           string
		id             string
		mockSetup      func(*MockSourceRepository)
		expectedStatus int
	}{
		{
			name: "get existing source",
			id:   testID,
			mockSetup: func(m *MockSourceRepository) {
				m.On("GetByID", mock.Anything, testID).Return(testSource, nil)
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "source not found",
			id:   "non-existent",
			mockSetup: func(m *MockSourceRepository) {
				m.On("GetByID", mock.Anything, "non-existent").Return(nil, errors.New("not found"))
			},
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := new(MockSourceRepository)
			tt.mockSetup(mockRepo)

			logger := testhelpers.NewTestLogger()
			// TODO: Handler needs interface injection for proper mocking
			db, _ := sql.Open("postgres", "host=localhost port=5432 user=postgres password=postgres dbname=gosources_test sslmode=disable")
			repo := repository.NewSourceRepository(db, logger)
			handler := handlers.NewSourceHandler(repo, logger)

			router := setupRouter(handler)

			req := httptest.NewRequest(http.MethodGet, "/sources/"+tt.id, http.NoBody)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			mockRepo.AssertExpectations(t)
		})
	}
}

func TestSourceHandler_List(t *testing.T) {
	t.Skip("Skipping handler tests until handlers use repository interfaces")
	testSources := []models.Source{
		{
			ID:        uuid.New().String(),
			Name:      "Source 1",
			URL:       "https://source1.com",
			RateLimit: "1s",
			MaxDepth:  2,
			Time:      models.StringArray{"09:00"},
			Selectors: models.SelectorConfig{
				Article: models.ArticleSelectors{Title: "h1"},
			},
			Enabled: true,
		},
		{
			ID:        uuid.New().String(),
			Name:      "Source 2",
			URL:       "https://source2.com",
			RateLimit: "1s",
			MaxDepth:  2,
			Time:      models.StringArray{"10:00"},
			Selectors: models.SelectorConfig{
				Article: models.ArticleSelectors{Title: "h1"},
			},
			Enabled: true,
		},
	}

	tests := []struct {
		name           string
		mockSetup      func(*MockSourceRepository)
		expectedStatus int
	}{
		{
			name: "list sources successfully",
			mockSetup: func(m *MockSourceRepository) {
				m.On("List", mock.Anything).Return(testSources, nil)
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "repository error",
			mockSetup: func(m *MockSourceRepository) {
				m.On("List", mock.Anything).Return(nil, errors.New("database error"))
			},
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := new(MockSourceRepository)
			tt.mockSetup(mockRepo)

			logger := testhelpers.NewTestLogger()
			// TODO: Handler needs interface injection for proper mocking
			db, _ := sql.Open("postgres", "host=localhost port=5432 user=postgres password=postgres dbname=gosources_test sslmode=disable")
			repo := repository.NewSourceRepository(db, logger)
			handler := handlers.NewSourceHandler(repo, logger)

			router := setupRouter(handler)

			req := httptest.NewRequest(http.MethodGet, "/sources", http.NoBody)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.expectedStatus == http.StatusOK {
				var response map[string]any
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)
				assert.Contains(t, response, "sources")
				assert.Contains(t, response, "count")
			}
			mockRepo.AssertExpectations(t)
		})
	}
}

func TestSourceHandler_GetCities(t *testing.T) {
	t.Skip("Skipping handler tests until handlers use repository interfaces")
	testCities := []models.City{
		{Name: "CityA", Index: "city_a_articles"},
		{Name: "CityB", Index: "city_b_articles"},
	}

	tests := []struct {
		name           string
		mockSetup      func(*MockSourceRepository)
		expectedStatus int
	}{
		{
			name: "get cities successfully",
			mockSetup: func(m *MockSourceRepository) {
				m.On("GetCities", mock.Anything).Return(testCities, nil)
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "repository error",
			mockSetup: func(m *MockSourceRepository) {
				m.On("GetCities", mock.Anything).Return(nil, errors.New("database error"))
			},
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := new(MockSourceRepository)
			tt.mockSetup(mockRepo)

			logger := testhelpers.NewTestLogger()
			// TODO: Handler needs interface injection for proper mocking
			db, _ := sql.Open("postgres", "host=localhost port=5432 user=postgres password=postgres dbname=gosources_test sslmode=disable")
			repo := repository.NewSourceRepository(db, logger)
			handler := handlers.NewSourceHandler(repo, logger)

			router := setupRouter(handler)

			req := httptest.NewRequest(http.MethodGet, "/cities", http.NoBody)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.expectedStatus == http.StatusOK {
				var response map[string]any
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)
				assert.Contains(t, response, "cities")
				assert.Contains(t, response, "count")
			}
			mockRepo.AssertExpectations(t)
		})
	}
}
