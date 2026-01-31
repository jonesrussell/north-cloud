package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jonesrussell/north-cloud/auth/internal/api"
	"github.com/jonesrussell/north-cloud/auth/internal/auth"
	"github.com/jonesrussell/north-cloud/auth/internal/config"
	infralogger "github.com/north-cloud/infrastructure/logger"
)

type mockLogger struct{}

func (m *mockLogger) Debug(_ string, _ ...infralogger.Field)         {}
func (m *mockLogger) Info(_ string, _ ...infralogger.Field)          {}
func (m *mockLogger) Warn(_ string, _ ...infralogger.Field)          {}
func (m *mockLogger) Error(_ string, _ ...infralogger.Field)         {}
func (m *mockLogger) Fatal(_ string, _ ...infralogger.Field)         {}
func (m *mockLogger) With(_ ...infralogger.Field) infralogger.Logger { return m }
func (m *mockLogger) Sync() error                                    { return nil }

func setupTestRouter(handler *api.AuthHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/api/v1/auth/login", handler.Login)
	return router
}

func TestAuthHandler_Login_Success(t *testing.T) {
	t.Helper()

	cfg := &config.Config{
		Auth: config.AuthConfig{
			Username:      "admin",
			Password:      "admin",
			JWTSecret:     "test-secret-key-32-chars-minimum",
			JWTExpiration: 24 * time.Hour,
		},
	}

	jwtMgr := auth.NewJWTManager(cfg.Auth.JWTSecret, cfg.Auth.JWTExpiration)
	handler := api.NewAuthHandler(cfg, jwtMgr, &mockLogger{})
	router := setupTestRouter(handler)

	reqBody := map[string]string{
		"username": "admin",
		"password": "admin",
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatalf("Failed to marshal request: %v", err)
	}

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Login() status = %d, want %d, body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	var response map[string]string
	if unmarshalErr := json.Unmarshal(w.Body.Bytes(), &response); unmarshalErr != nil {
		t.Fatalf("Failed to unmarshal response: %v", unmarshalErr)
	}

	if response["token"] == "" {
		t.Error("Login() expected token in response")
	}
}

func TestAuthHandler_Login_InvalidCredentials(t *testing.T) {
	t.Helper()

	cfg := &config.Config{
		Auth: config.AuthConfig{
			Username:      "admin",
			Password:      "admin",
			JWTSecret:     "test-secret-key-32-chars-minimum",
			JWTExpiration: 24 * time.Hour,
		},
	}

	jwtMgr := auth.NewJWTManager(cfg.Auth.JWTSecret, cfg.Auth.JWTExpiration)
	handler := api.NewAuthHandler(cfg, jwtMgr, &mockLogger{})
	router := setupTestRouter(handler)

	testCases := []struct {
		name     string
		username string
		password string
	}{
		{"wrong username", "wrong", "admin"},
		{"wrong password", "admin", "wrong"},
		{"both wrong", "wrong", "wrong"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			reqBody := map[string]string{
				"username": tc.username,
				"password": tc.password,
			}
			body, err := json.Marshal(reqBody)
			if err != nil {
				t.Fatalf("Failed to marshal request: %v", err)
			}

			w := httptest.NewRecorder()
			req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(w, req)

			if w.Code != http.StatusUnauthorized {
				t.Errorf("Login() status = %d, want %d", w.Code, http.StatusUnauthorized)
			}
		})
	}
}

func TestAuthHandler_Login_MalformedRequest(t *testing.T) {
	t.Helper()

	cfg := &config.Config{
		Auth: config.AuthConfig{
			Username:      "admin",
			Password:      "admin",
			JWTSecret:     "test-secret-key-32-chars-minimum",
			JWTExpiration: 24 * time.Hour,
		},
	}

	jwtMgr := auth.NewJWTManager(cfg.Auth.JWTSecret, cfg.Auth.JWTExpiration)
	handler := api.NewAuthHandler(cfg, jwtMgr, &mockLogger{})
	router := setupTestRouter(handler)

	testCases := []struct {
		name string
		body string
	}{
		{"empty body", ""},
		{"invalid json", "{invalid}"},
		{"missing username", `{"password": "admin"}`},
		{"missing password", `{"username": "admin"}`},
		{"empty fields", `{"username": "", "password": ""}`},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(tc.body))
			req.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("Login() status = %d, want %d, body: %s", w.Code, http.StatusBadRequest, w.Body.String())
			}
		})
	}
}
