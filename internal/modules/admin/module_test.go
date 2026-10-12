package admin_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"antdv-next-admin-go/internal/httpserver/middleware"
	"antdv-next-admin-go/internal/modules/admin"
	authcode "antdv-next-admin-go/internal/modules/auth/errcode"
	"antdv-next-admin-go/internal/modules/auth/model"
)

type authenticator struct{}

func (authenticator) Authenticate(_ context.Context, _ string) (model.Principal, error) {
	return model.Principal{UserID: 7, Username: "operator"}, nil
}

type authorizer struct {
	allowed bool
	userID  int64
	method  string
	route   string
}

func (a *authorizer) Authorize(_ context.Context, userID int64, method, route string) (bool, error) {
	a.userID, a.method, a.route = userID, method, route
	return a.allowed, nil
}

func TestPingRequiresPermissionAndReturnsPong(t *testing.T) {
	gin.SetMode(gin.TestMode)
	checks := &authorizer{allowed: true}
	router := gin.New()
	private := router.Group("/api", middleware.RequireLogin(authenticator{}))
	admin.NewModule().RegisterRoutes(private, checks)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/admin/ping", nil)
	request.Header.Set("Authorization", "Bearer test-token")
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if checks.userID != 7 || checks.method != http.MethodGet || checks.route != "/api/admin/ping" {
		t.Fatalf("unexpected authorization check: %+v", checks)
	}
	var body struct {
		Code int `json:"code"`
		Data struct {
			Message string `json:"message"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Code != 0 || body.Data.Message != "pong" {
		t.Fatalf("unexpected response: %+v", body)
	}
}

func TestPingReturnsForbiddenWithoutPermission(t *testing.T) {
	gin.SetMode(gin.TestMode)
	checks := &authorizer{}
	router := gin.New()
	private := router.Group("/api", middleware.RequireLogin(authenticator{}))
	admin.NewModule().RegisterRoutes(private, checks)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/admin/ping", nil)
	request.Header.Set("Authorization", "Bearer test-token")
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusForbidden, recorder.Body.String())
	}
	var body struct {
		Code int `json:"code"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Code != authcode.PermissionDenied {
		t.Fatalf("business code = %d, want %d", body.Code, authcode.PermissionDenied)
	}
}
