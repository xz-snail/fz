package global

import (
	"ctaf-fz/config"

	"go.uber.org/zap"
)

var (
	// Config 保存当前进程加载后的配置。
	Config *config.Config
	// Logger 保存当前进程使用的结构化日志实例。
	Logger = zap.NewNop()
)
