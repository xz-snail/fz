package middleware

import (
	"fmt"
	"net/http"
	"time"

	"ctaf-fz/global"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func GinRecovery(stack bool) gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, recovered any) {
		fields := []zap.Field{
			zap.Any("error", recovered),
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
		}
		if stack {
			fields = append(fields, zap.Stack("stack"))
		}
		global.Logger.Error("panic recovered", fields...)
		c.AbortWithStatus(http.StatusInternalServerError)
	})
}

func GinLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		latency := time.Since(start)
		global.Logger.Info(fmt.Sprintf("[GIN] %3d | %13v | %15s | %-7s %q",
			c.Writer.Status(),
			latency,
			c.ClientIP(),
			c.Request.Method,
			c.Request.URL.RequestURI(),
		))
	}
}
