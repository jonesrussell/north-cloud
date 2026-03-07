package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func newTestContext(rawQuery string) *gin.Context {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req, _ := http.NewRequest(http.MethodGet, "/?"+rawQuery, nil)
	c.Request = req
	return c
}

func TestParseFilters_TopicsArrayFormat(t *testing.T) {
	t.Helper()

	c := newTestContext("topics%5B%5D=indigenous&topics%5B%5D=crime")
	filters := parseFilters(c)

	if len(filters.Topics) != 2 {
		t.Fatalf("expected 2 topics, got %d: %v", len(filters.Topics), filters.Topics)
	}
	if filters.Topics[0] != "indigenous" {
		t.Errorf("expected topics[0]=%q, got %q", "indigenous", filters.Topics[0])
	}
	if filters.Topics[1] != "crime" {
		t.Errorf("expected topics[1]=%q, got %q", "crime", filters.Topics[1])
	}
}

func TestParseFilters_TopicsCommaFormat(t *testing.T) {
	t.Helper()

	c := newTestContext("topics=indigenous,crime")
	filters := parseFilters(c)

	if len(filters.Topics) != 2 {
		t.Fatalf("expected 2 topics, got %d: %v", len(filters.Topics), filters.Topics)
	}
}

func TestParseFilters_TopicsEmpty(t *testing.T) {
	t.Helper()

	c := newTestContext("q=technology")
	filters := parseFilters(c)

	if len(filters.Topics) != 0 {
		t.Errorf("expected no topics, got %v", filters.Topics)
	}
}
