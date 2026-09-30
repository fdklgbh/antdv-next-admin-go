package response

import (
	stderrors "errors"
	"net/http"
	"time"

	commoncode "go-gin-template/internal/shared/errcode"
	apperrors "go-gin-template/internal/shared/errors"

	"github.com/gin-gonic/gin"
)

const RequestIDKey = "request.id"

type Body struct {
	Code      int    `json:"code"`
	Message   string `json:"message"`
	Data      any    `json:"data"`
	RequestID string `json:"requestId"`
	Timestamp int64  `json:"timestamp"`
}

func Success(c *gin.Context, data any) {
	c.JSON(http.StatusOK, newBody(c, commoncode.OK, "success", data))
}
func Created(c *gin.Context, data any) {
	c.JSON(http.StatusCreated, newBody(c, commoncode.OK, "success", data))
}
func NoContent(c *gin.Context) { c.Status(http.StatusNoContent) }
func Error(c *gin.Context, err error) {
	var appErr *apperrors.AppError
	if !stderrors.As(err, &appErr) {
		appErr = apperrors.New(commoncode.InternalError, http.StatusInternalServerError, "internal server error", err)
	}
	c.JSON(appErr.HTTPStatus, newBody(c, appErr.Code, appErr.Message, nil))
}
func Abort(c *gin.Context, err error) { Error(c, err); c.Abort() }
func ValidationError(c *gin.Context, data any) {
	c.JSON(http.StatusBadRequest, newBody(c, commoncode.ValidationFailed, "validation failed", data))
}
func RequestID(c *gin.Context) string {
	value, exists := c.Get(RequestIDKey)
	if !exists {
		return ""
	}
	requestID, _ := value.(string)
	return requestID
}
func newBody(c *gin.Context, code int, message string, data any) Body {
	return Body{Code: code, Message: message, Data: data, RequestID: RequestID(c), Timestamp: time.Now().UnixMilli()}
}
