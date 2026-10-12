package account_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"antdv-next-admin-go/internal/httpserver/middleware"
	"antdv-next-admin-go/internal/modules/account"
	"antdv-next-admin-go/internal/modules/auth/model"
)

type authenticator struct{ principal model.Principal }

func (a authenticator) Authenticate(_ context.Context, _ string) (model.Principal, error) {
	return a.principal, nil
}

func TestMeReturnsAuthenticatedPrincipal(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	module := account.NewModule()
	router.GET("/api/account/me", middleware.RequireLogin(authenticator{principal: model.Principal{UserID: 42, Username: "test-user"}}), module.Handler.Me)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/account/me", nil)
	request.Header.Set("Authorization", "Bearer test-token")
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	var body struct {
		Code int             `json:"code"`
		Data model.Principal `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Code != 0 || body.Data.UserID != 42 || body.Data.Username != "test-user" {
		t.Fatalf("unexpected response: %+v", body)
	}
}

func TestMeRequiresAuthentication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	module := account.NewModule()
	router.GET("/api/account/me", middleware.RequireLogin(authenticator{}), module.Handler.Me)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/account/me", nil))
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusUnauthorized, recorder.Body.String())
	}
	if strings.Contains(recorder.Body.String(), `"data":{"userId":0`) {
		t.Fatalf("unauthenticated request returned an empty principal: %s", recorder.Body.String())
	}
}
