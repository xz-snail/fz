package initialize

import (
	"flag"

	"ctaf-fz/config"
	"ctaf-fz/global"

	"go.uber.org/zap"
)

// InitApp 解析启动参数并加载全局配置。
func InitApp() (*config.Config, error) {
	configPath := flag.String("config", "config.yaml", "path to config yaml")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		return nil, err
	}
	global.Config = cfg
	logger, err := InitLogger(cfg.Log)
	if err != nil {
		return nil, err
	}
	global.Logger = logger
	zap.ReplaceGlobals(logger)
	return cfg, nil
}
