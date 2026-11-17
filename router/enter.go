package router

import fzrouter "ctaf-fz/router/fz"

// RouterGroupApp 是项目路由组入口。
var RouterGroupApp = new(RouterGroup)

// RouterGroup 汇总各业务模块路由。
type RouterGroup struct {
	FZ fzrouter.RouterGroup
}
