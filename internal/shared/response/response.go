package response

import (
	stderrors "errors"
	"net/http"
	"time"

	commoncode "antdv-next-admin-go/internal/shared/errcode"
	apperrors "antdv-next-admin-go/internal/shared/errors"

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
	c.JSON(http.StatusOK, newBody(c, commoncode.OK, "操作成功", data))
}
func Created(c *gin.Context, data any) {
	c.JSON(http.StatusCreated, newBody(c, commoncode.OK, "操作成功", data))
}
func NoContent(c *gin.Context) { c.Status(http.StatusNoContent) }
func Error(c *gin.Context, err error) {
	var appErr *apperrors.AppError
	if !stderrors.As(err, &appErr) {
		appErr = apperrors.New(commoncode.InternalError, http.StatusInternalServerError, "服务器内部错误，请稍后重试", err)
	}
	c.JSON(appErr.HTTPStatus, newBody(c, appErr.Code, appErr.Message, nil))
}
func Abort(c *gin.Context, err error) { Error(c, err); c.Abort() }
func ValidationError(c *gin.Context, data any) {
	c.JSON(http.StatusBadRequest, newBody(c, commoncode.ValidationFailed, "参数无效，请检查输入", data))
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
