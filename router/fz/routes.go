package fz

import (
	fzapi "ctaf-fz/api/v1/fz"

	"github.com/gin-gonic/gin"
)

// InitLegacyRoutes 注册兼容 FZ.php 的根路径接口。
func (r RouterGroup) InitLegacyRoutes(engine *gin.Engine, h fzapi.FlowApi) {
	engine.GET("/healthz", h.Health)
	engine.POST("/statusreport", h.StatusReport)
	engine.POST("/rule/increment", h.RuleIncrement)
	engine.POST("/rule/upgrade", h.RuleUpgrade)
	engine.POST("/upgrade/redirect", h.UpgradeRedirect)
}
