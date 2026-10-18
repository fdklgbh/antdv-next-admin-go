package handler

import (
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mileusna/useragent"

	"antdv-next-admin-go/internal/httpserver/middleware"
	"antdv-next-admin-go/internal/infrastructure/logger"
	"antdv-next-admin-go/internal/modules/system/dto"
	"antdv-next-admin-go/internal/modules/system/model"
	"antdv-next-admin-go/internal/modules/system/repository"
	"antdv-next-admin-go/internal/modules/system/service"
	commoncode "antdv-next-admin-go/internal/shared/errcode"
	apperrors "antdv-next-admin-go/internal/shared/errors"
	"antdv-next-admin-go/internal/shared/response"
)

type Handler struct {
	service *service.Service
}

func New(s *service.Service) *Handler { return &Handler{service: s} }
func (h *Handler) fail(c *gin.Context, err error) {
	status, code, message := 500, commoncode.InternalError, "系统操作失败"
	switch {
	case errors.Is(err, repository.ErrNotFound):
		status, code, message = 404, commoncode.NotFound, "资源不存在"
	case errors.Is(err, repository.ErrInvalid):
		status, code, message = 400, commoncode.ValidationFailed, "参数无效"
	case errors.Is(err, repository.ErrConflict):
		status, code, message = 409, commoncode.ResourceConflict, "资源重复或仍被引用"
	case errors.Is(err, repository.ErrProtected):
		status, code, message = 403, commoncode.ResourceProtected, "受保护的资源，不允许此操作"
	default:
		logger.FromContext(c.Request.Context()).Error("system_request_failed", logger.Err(err))
	}
	response.Error(c, apperrors.New(code, status, message, nil))
}
func (h *Handler) List(kind string, all bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		q := dto.Query{Page: 1, PageSize: 10}
		if c.ShouldBindQuery(&q) != nil {
			response.ValidationError(c, nil)
			return
		}
		if code := c.Param("typeCode"); code != "" {
			q.TypeCode = code
		}
		out, err := h.service.List(c.Request.Context(), kind, q, all)
		if err != nil {
			h.fail(c, err)
			return
		}
		if all {
			response.Success(c, out.List)
		} else {
			response.Success(c, out)
		}
	}
}
func (h *Handler) ConfigGroups(c *gin.Context) {
	out, err := h.service.ConfigGroups(c.Request.Context())
	if err != nil {
		h.fail(c, err)
		return
	}
	response.Success(c, out)
}
func (h *Handler) ConfigByKey(c *gin.Context) {
	out, err := h.service.ConfigByKey(c.Request.Context(), c.Param("key"))
	if err != nil {
		h.fail(c, err)
		return
	}
	response.Success(c, out)
}
func (h *Handler) Save(kind string) gin.HandlerFunc {
	return func(c *gin.Context) {
		var input any
		switch kind {
		case "config":
			input = &model.Config{}
		case "dict_type":
			input = &model.DictType{}
		case "dict_data":
			input = &model.DictData{}
		default:
			h.fail(c, repository.ErrInvalid)
			return
		}
		if c.ShouldBindJSON(input) != nil {
			response.ValidationError(c, nil)
			return
		}
		out, err := h.service.Save(c.Request.Context(), c.Param("id"), input)
		if err != nil {
			h.fail(c, err)
			return
		}
		response.Success(c, out)
	}
}
func (h *Handler) Delete(kind string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := h.service.Delete(c.Request.Context(), kind, c.Param("id")); err != nil {
			h.fail(c, err)
			return
		}
		response.Success(c, nil)
	}
}
func (h *Handler) Clear(kind string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := h.service.ClearLogs(c.Request.Context(), kind); err != nil {
			h.fail(c, err)
			return
		}
		response.Success(c, nil)
	}
}
func (h *Handler) Upload(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 11*1024*1024)
	if err := c.Request.ParseMultipartForm(11 * 1024 * 1024); err != nil {
		response.ValidationError(c, nil)
		return
	}
	if c.Request.MultipartForm != nil {
		defer c.Request.MultipartForm.RemoveAll()
	}
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		response.ValidationError(c, nil)
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, 10*1024*1024+1))
	if err != nil {
		h.fail(c, err)
		return
	}
	principal, _ := middleware.CurrentPrincipal(c)
	out, err := h.service.Upload(c.Request.Context(), header.Filename, principal.Username, data)
	if err != nil {
		h.fail(c, err)
		return
	}
	response.Success(c, out)
}
func (h *Handler) Download(c *gin.Context) {
	file, err := h.service.File(c.Request.Context(), c.Param("id"))
	if err != nil {
		h.fail(c, err)
		return
	}
	c.Header("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": file.OriginalName}))
	c.Header("Cache-Control", "no-store")
	c.Data(200, "application/octet-stream", file.Data)
}
func (h *Handler) Audit() gin.HandlerFunc {
	return func(c *gin.Context) {
		started := time.Now()
		c.Next()
		route := c.FullPath()
		if route == "" || c.Request.Method == http.MethodGet || strings.HasPrefix(route, "/api/log/") || route == "/api/auth/refresh" {
			return
		}
		principal, _ := middleware.CurrentPrincipal(c)
		username := principal.Username
		kind := "operation"
		action := "other"
		if route == "/api/auth/login" {
			kind = "login"
			action = "login"
			username = c.GetString("audit.username")
		}
		if username == "" && kind != "login" {
			return
		}
		switch c.Request.Method {
		case "POST":
			if action == "other" {
				action = "create"
			}
		case "PUT", "PATCH":
			action = "update"
		case "DELETE":
			action = "delete"
		}
		if route == "/api/auth/logout" {
			action = "logout"
		}
		state := "success"
		if c.Writer.Status() >= 400 {
			state = "fail"
		}
		module := strings.Split(strings.TrimPrefix(route, "/api/"), "/")[0]
		browser, os := "", ""
		if kind == "login" {
			browser, os = parseClientAgent(c.Request.UserAgent())
		}
		// 只记录账号、路径模板和结果，绝不记录请求体、Token、密码或查询参数。
		row := model.Log{Kind: kind, Username: username, Module: module, Action: action, Description: route, Method: c.Request.Method, URL: route, IP: c.ClientIP(), Browser: browser, OS: os, Status: state, Message: fmt.Sprintf("HTTP %d", c.Writer.Status()), Duration: time.Since(started).Milliseconds()}
		if err := h.service.Audit(c.Request.Context(), row); err != nil {
			logger.FromContext(c.Request.Context()).Error("audit_persist_failed", logger.Err(err))
		}
	}
}

func parseClientAgent(value string) (string, string) {
	const maxUserAgentLength = 1024
	if value == "" || len(value) > maxUserAgentLength {
		return "", ""
	}
	parsed := useragent.Parse(value)
	browser := ""
	if parsed.Name != "" && parsed.Name != parsed.String {
		browser = parsed.Name
		if version := parsed.VersionNoShort(); version != "" {
			browser += " " + version
		}
	}
	operatingSystem := parsed.OS
	if version := parsed.OSVersionNoShort(); version != "" {
		operatingSystem += " " + version
	}
	return browser, operatingSystem
}

func (h *Handler) EnabledDictionaries(c *gin.Context) {
	out, err := h.service.List(c.Request.Context(), "dict_data", dto.Query{Page: 1, PageSize: 10000, Status: "enabled"}, true)
	if err != nil {
		h.fail(c, err)
		return
	}
	response.Success(c, out.List)
}
