package management

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestMiddlewareAndRequireRoles_SuperAdminAndOperator(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("super admin can access admin route", func(t *testing.T) {
		t.Setenv("MANAGEMENT_PASSWORD", "super-secret")
		t.Setenv("MANAGEMENT_OPERATOR_PASSWORD", "")

		h := NewHandlerWithoutConfigFilePath(nil, nil)
		router := gin.New()
		group := router.Group("/v0/management")
		group.Use(h.Middleware())
		group.GET("/admin-only", h.RequireRoles("super_admin"), func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"role": CurrentManagementRole(c)})
		})

		req := httptest.NewRequest(http.MethodGet, "/v0/management/admin-only", nil)
		req.Header.Set("Authorization", "Bearer super-secret")
		req.RemoteAddr = "127.0.0.1:12345"
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("operator can access operator route", func(t *testing.T) {
		t.Setenv("MANAGEMENT_PASSWORD", "super-secret")
		t.Setenv("MANAGEMENT_OPERATOR_PASSWORD", "operator-secret")

		h := NewHandlerWithoutConfigFilePath(nil, nil)
		router := gin.New()
		group := router.Group("/v0/management")
		group.Use(h.Middleware())
		group.GET("/operator-only", h.RequireRoles("super_admin", "file_operator"), func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"role": CurrentManagementRole(c)})
		})

		req := httptest.NewRequest(http.MethodGet, "/v0/management/operator-only", nil)
		req.Header.Set("Authorization", "Bearer operator-secret")
		req.RemoteAddr = "127.0.0.1:12345"
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("operator cannot access admin route", func(t *testing.T) {
		t.Setenv("MANAGEMENT_PASSWORD", "super-secret")
		t.Setenv("MANAGEMENT_OPERATOR_PASSWORD", "operator-secret")

		h := NewHandlerWithoutConfigFilePath(nil, nil)
		router := gin.New()
		group := router.Group("/v0/management")
		group.Use(h.Middleware())
		group.GET("/admin-only", h.RequireRoles("super_admin"), func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"role": CurrentManagementRole(c)})
		})

		req := httptest.NewRequest(http.MethodGet, "/v0/management/admin-only", nil)
		req.Header.Set("Authorization", "Bearer operator-secret")
		req.RemoteAddr = "127.0.0.1:12345"
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected status 403, got %d body=%s", rec.Code, rec.Body.String())
		}
	})
}
