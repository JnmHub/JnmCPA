package management

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/authdeletestats"
)

func TestGetAuthDeleteStats(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest(http.MethodGet, "/v0/management/auth-delete-stats?range=24h&bucket=1h", nil)
	ctx.Request = req

	stats := authdeletestats.NewManager(24*time.Hour, 32)
	stats.Record(authdeletestats.Event{
		Timestamp:  time.Now().UTC().Add(-30 * time.Minute),
		StatusCode: authdeletestats.StatusUnauthorized,
	})
	stats.Record(authdeletestats.Event{
		Timestamp:  time.Now().UTC().Add(-10 * time.Minute),
		StatusCode: authdeletestats.StatusTooMany,
	})

	handler := NewHandlerWithoutConfigFilePath(nil, nil)
	handler.SetAuthDeleteStatistics(stats)

	handler.GetAuthDeleteStats(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	body := recorder.Body.String()
	if !contains(body, `"status_401":1`) {
		t.Fatalf("response missing 401 total: %s", body)
	}
	if !contains(body, `"status_429":1`) {
		t.Fatalf("response missing 429 total: %s", body)
	}
}

func contains(haystack, needle string) bool {
	return len(needle) == 0 || (len(haystack) >= len(needle) && stringContains(haystack, needle))
}

func stringContains(haystack, needle string) bool {
	for idx := 0; idx+len(needle) <= len(haystack); idx++ {
		if haystack[idx:idx+len(needle)] == needle {
			return true
		}
	}
	return false
}
