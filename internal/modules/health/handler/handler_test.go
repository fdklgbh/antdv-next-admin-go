package handler_test

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"antdv-next-admin-go/internal/infrastructure/logger"
	"antdv-next-admin-go/internal/modules/health/handler"
	commoncode "antdv-next-admin-go/internal/shared/errcode"
)

type pingConnector struct{ err error }

func (c pingConnector) Connect(context.Context) (driver.Conn, error) {
	return pingConnection{err: c.err}, nil
}
func (pingConnector) Driver() driver.Driver { return pingDriver{} }

type pingDriver struct{}

func (pingDriver) Open(string) (driver.Conn, error) { return pingConnection{}, nil }

type pingConnection struct{ err error }

func (pingConnection) Prepare(string) (driver.Stmt, error) { return nil, errors.New("not supported") }
func (pingConnection) Close() error                        { return nil }
func (pingConnection) Begin() (driver.Tx, error)           { return nil, errors.New("not supported") }
func (c pingConnection) Ping(context.Context) error        { return c.err }

type cachePinger struct {
	err   error
	calls int
}

func (p *cachePinger) Ping(context.Context) error {
	p.calls++
	return p.err
}

type testLogger struct{}

func (testLogger) Debug(string, ...logger.Field)        {}
func (testLogger) Info(string, ...logger.Field)         {}
func (testLogger) Warn(string, ...logger.Field)         {}
func (testLogger) Error(string, ...logger.Field)        {}
func (l testLogger) With(...logger.Field) logger.Logger { return l }

func TestLiveReturnsHealthy(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/health/live", handler.New(nil, nil).Live)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/health/live", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
}

func TestReadyChecksDatabaseBeforeCache(t *testing.T) {
	databaseErr := errors.New("database unavailable")
	cache := &cachePinger{}
	router, closeDB := readyRouter(t, databaseErr, cache)
	defer closeDB()

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/health/ready", nil))
	assertUnavailable(t, recorder)
	if cache.calls != 0 {
		t.Fatalf("cache ping count = %d, want 0 when database is unavailable", cache.calls)
	}
}

func TestReadyReportsCacheFailure(t *testing.T) {
	cache := &cachePinger{err: errors.New("cache unavailable")}
	router, closeDB := readyRouter(t, nil, cache)
	defer closeDB()

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/health/ready", nil))
	assertUnavailable(t, recorder)
	if cache.calls != 1 {
		t.Fatalf("cache ping count = %d, want 1", cache.calls)
	}
}

func TestReadySucceedsWhenDependenciesAreAvailable(t *testing.T) {
	cache := &cachePinger{}
	router, closeDB := readyRouter(t, nil, cache)
	defer closeDB()

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/health/ready", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if cache.calls != 1 {
		t.Fatalf("cache ping count = %d, want 1", cache.calls)
	}
}

func readyRouter(t *testing.T, databaseErr error, cache *cachePinger) (*gin.Engine, func()) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db := sql.OpenDB(pingConnector{err: databaseErr})
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Request = c.Request.WithContext(logger.WithLogger(c.Request.Context(), testLogger{}))
		c.Next()
	})
	router.GET("/health/ready", handler.New(db, cache).Ready)
	return router, func() { _ = db.Close() }
}

func assertUnavailable(t *testing.T, recorder *httptest.ResponseRecorder) {
	t.Helper()
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusServiceUnavailable, recorder.Body.String())
	}
	var body struct {
		Code int `json:"code"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Code != commoncode.ServiceUnavailable {
		t.Fatalf("business code = %d, want %d", body.Code, commoncode.ServiceUnavailable)
	}
}
