package fz

import "ctaf-fz/service"

// ApiGroup 注册 FZ 接口处理对象。
type ApiGroup struct {
	FlowApi
}

// 暴漏service
var (
	flowService = &service.ServiceGroupApp.FZServiceGroup.FlowService
)
