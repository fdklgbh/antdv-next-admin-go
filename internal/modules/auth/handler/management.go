package handler

import (
	"antdv-next-admin-go/internal/modules/auth/dto"
	"antdv-next-admin-go/internal/modules/auth/model"
	"antdv-next-admin-go/internal/shared/contextx"
	commoncode "antdv-next-admin-go/internal/shared/errcode"
	apperrors "antdv-next-admin-go/internal/shared/errors"
	"antdv-next-admin-go/internal/shared/response"
	"errors"
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
)

func (h *Handler) managementFail(c *gin.Context, err error) {
	switch {
	case errors.Is(err, model.ErrNotFound):
		response.Error(c, apperrors.New(commoncode.NotFound, 404, "资源不存在", nil))
	case errors.Is(err, model.ErrInvalid):
		response.Error(c, apperrors.New(commoncode.ValidationFailed, 400, "参数无效，请检查必填项、关联数据及不可修改字段", nil))
	case errors.Is(err, model.ErrConflict):
		response.Error(c, apperrors.New(commoncode.ResourceConflict, 409, "资源重复或仍被引用，请先解除关联", nil))
	case errors.Is(err, model.ErrProtected):
		response.Error(c, apperrors.New(commoncode.ResourceProtected, 403, "受保护的资源，不允许此操作", nil))
	default:
		h.fail(c, err)
	}
}
func pageQuery(c *gin.Context) (dto.PageQuery, bool) {
	q := dto.PageQuery{Current: 1, PageSize: 10}
	if err := c.ShouldBindQuery(&q); err != nil {
		response.ValidationError(c, nil)
		return q, false
	}
	return q, true
}
func numericID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.ValidationError(c, nil)
		return 0, false
	}
	return id, true
}
func (h *Handler) Users(c *gin.Context) {
	q, ok := pageQuery(c)
	if !ok {
		return
	}
	out, err := h.service.ListUsers(c.Request.Context(), q)
	if err != nil {
		h.managementFail(c, err)
		return
	}
	response.Success(c, out)
}
func (h *Handler) User(c *gin.Context) {
	id, ok := numericID(c)
	if !ok {
		return
	}
	out, err := h.service.ManagedUser(c.Request.Context(), id)
	if err != nil {
		h.managementFail(c, err)
		return
	}
	response.Success(c, out)
}
func (h *Handler) SaveUser(c *gin.Context) {
	var id int64
	if c.Request.Method == http.MethodPut {
		var ok bool
		id, ok = numericID(c)
		if !ok {
			return
		}
	}
	var in dto.UserInput
	if c.ShouldBindJSON(&in) != nil {
		response.ValidationError(c, nil)
		return
	}
	actor, _ := contextx.UserID(c.Request.Context())
	out, err := h.service.SaveUser(c.Request.Context(), actor, id, in)
	if err != nil {
		h.managementFail(c, err)
		return
	}
	response.Success(c, out)
}
func (h *Handler) DeleteUser(c *gin.Context) {
	id, ok := numericID(c)
	if !ok {
		return
	}
	actor, _ := contextx.UserID(c.Request.Context())
	if err := h.service.DeleteUser(c.Request.Context(), actor, id); err != nil {
		h.managementFail(c, err)
		return
	}
	response.Success(c, nil)
}
func (h *Handler) Roles(c *gin.Context) {
	q, ok := pageQuery(c)
	if !ok {
		return
	}
	out, err := h.service.ListRoles(c.Request.Context(), q)
	if err != nil {
		h.managementFail(c, err)
		return
	}
	response.Success(c, out)
}
func (h *Handler) Role(c *gin.Context) {
	id, ok := numericID(c)
	if !ok {
		return
	}
	out, err := h.service.Role(c.Request.Context(), id)
	if err != nil {
		h.managementFail(c, err)
		return
	}
	response.Success(c, out)
}
func (h *Handler) SaveRole(c *gin.Context) {
	var id int64
	if c.Request.Method == http.MethodPut {
		var ok bool
		id, ok = numericID(c)
		if !ok {
			return
		}
	}
	var in dto.RoleInput
	if c.ShouldBindJSON(&in) != nil {
		response.ValidationError(c, nil)
		return
	}
	actor, _ := contextx.UserID(c.Request.Context())
	out, err := h.service.SaveRole(c.Request.Context(), actor, id, in)
	if err != nil {
		h.managementFail(c, err)
		return
	}
	response.Success(c, out)
}
func (h *Handler) DeleteRole(c *gin.Context) {
	id, ok := numericID(c)
	if !ok {
		return
	}
	if err := h.service.DeleteRole(c.Request.Context(), id); err != nil {
		h.managementFail(c, err)
		return
	}
	response.Success(c, nil)
}
func (h *Handler) Permissions(c *gin.Context) {
	q, ok := pageQuery(c)
	if !ok {
		return
	}
	out, err := h.service.Permissions(c.Request.Context(), q, true)
	if err != nil {
		h.managementFail(c, err)
		return
	}
	response.Success(c, out)
}
func (h *Handler) Permission(c *gin.Context) {
	out, err := h.service.Permission(c.Request.Context(), c.Param("id"))
	if err != nil {
		h.managementFail(c, err)
		return
	}
	response.Success(c, out)
}
func (h *Handler) SavePermission(c *gin.Context) {
	var in dto.PermissionInfo
	if c.ShouldBindJSON(&in) != nil {
		response.ValidationError(c, nil)
		return
	}
	out, err := h.service.SavePermission(c.Request.Context(), c.Param("id"), in)
	if err != nil {
		h.managementFail(c, err)
		return
	}
	response.Success(c, out)
}
func (h *Handler) DeletePermission(c *gin.Context) {
	if err := h.service.DeletePermission(c.Request.Context(), c.Param("id")); err != nil {
		h.managementFail(c, err)
		return
	}
	response.Success(c, nil)
}
func (h *Handler) UserPermissions(c *gin.Context) {
	id, _ := contextx.UserID(c.Request.Context())
	info, err := h.service.UserInfo(c.Request.Context(), id)
	if err != nil {
		h.managementFail(c, err)
		return
	}
	response.Success(c, info.Permissions)
}
func (h *Handler) ChangePassword(c *gin.Context) {
	var in dto.PasswordInput
	if c.ShouldBindJSON(&in) != nil {
		response.ValidationError(c, nil)
		return
	}
	id, _ := contextx.UserID(c.Request.Context())
	if err := h.service.ChangePassword(c.Request.Context(), id, in); err != nil {
		h.managementFail(c, err)
		return
	}
	response.Success(c, nil)
}
