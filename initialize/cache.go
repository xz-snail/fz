package initialize

import (
	"ctaf-fz/config"
	fzservice "ctaf-fz/service/fz"
)

// InitCache 创建规则状态缓存。
func InitCache(cfg *config.Config) *fzservice.Store {
	return fzservice.NewStore()
}
