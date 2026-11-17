package v1

import fzApi "ctaf-fz/api/v1/fz"

// ApiGroupApp 是接口层分组入口。
var ApiGroupApp = new(ApiGroup)

// ApiGroup 汇总各业务模块接口。
type ApiGroup struct {
	FZApiGroup fzApi.ApiGroup
}
