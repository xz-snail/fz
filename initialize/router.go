package initialize

import (
	"ctaf-fz/config"
	"ctaf-fz/middleware"
	"ctaf-fz/router"

	"github.com/gin-gonic/gin"
)

// Routers 创建 Gin 引擎并注册 legacy 路由。
func Routers(cfg *config.Config, app *App) *gin.Engine {
	engine := gin.New()
	engine.Use(middleware.GinRecovery(true))
	if cfg.Log.GinRequests {
		engine.Use(middleware.GinLogger())
	}

	fzRouter := router.RouterGroupApp.FZ
	fzRouter.InitLegacyRoutes(engine, app.ApiGroupApp.FZApiGroup.FlowApi)
	return engine
}

// InitServer 初始化缓存、依赖和 Gin 路由。
func InitServer(cfg *config.Config) (*gin.Engine, *App, error) {
	store := InitCache(cfg)
	app := WireApp(cfg, store)
	return Routers(cfg, app), app, nil
}
