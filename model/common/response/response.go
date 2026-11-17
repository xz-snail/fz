package response

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	// ERROR 表示业务失败状态码。
	ERROR = 400
	// SUCCESS 表示业务成功状态码。
	SUCCESS = 200
)

// MessageResponse 是 FZ 接口统一响应结构。
type MessageResponse struct {
	Code      int    `json:"code"`
	Message   string `json:"message"`
	Body      any    `json:"body"`
	Timestamp int64  `json:"timestamp"`
}

// Result 输出 FZ 统一响应。
func Result(c *gin.Context, code int, message string, body any) {
	c.JSON(http.StatusOK, MessageResponse{
		Code:      code,
		Message:   message,
		Body:      body,
		Timestamp: time.Now().Unix(),
	})
}

// ResultWithStatus 输出已有响应体，并保留调用方指定的 HTTP 状态码。
func ResultWithStatus(c *gin.Context, status int, payload any) {
	c.JSON(status, payload)
}

// Ok 输出 FZ 成功响应。
func Ok(c *gin.Context) {
	OkWithBody(c, true)
}

// OkWithBody 输出带自定义 body 的 FZ 成功响应。
func OkWithBody(c *gin.Context, body any) {
	Result(c, SUCCESS, "成功", body)
}

// Fail 输出 FZ 失败响应。
func Fail(c *gin.Context, message string) {
	FailWithBody(c, message, false)
}

// FailWithBody 输出带自定义 body 的 FZ 失败响应。
func FailWithBody(c *gin.Context, message string, body any) {
	Result(c, ERROR, message, body)
}
