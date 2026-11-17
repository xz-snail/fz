package service

import fzService "ctaf-fz/service/fz"

// ServiceGroupApp 是业务层分组入口。
var ServiceGroupApp = new(ServiceGroup)

// ServiceGroup 汇总各业务模块服务。
type ServiceGroup struct {
	FZServiceGroup fzService.ServiceGroup
}
