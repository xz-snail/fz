package initialize

import (
	v1 "ctaf-fz/api/v1"
	"ctaf-fz/config"
	"ctaf-fz/jobs"
	rulejob "ctaf-fz/jobs/rule"
	"ctaf-fz/service"
	fzservice "ctaf-fz/service/fz"
)

// App 汇总服务运行期依赖，便于统一启动和关闭。
type App struct {
	ApiGroupApp     *v1.ApiGroup
	ServiceGroupApp *service.ServiceGroup
	CleanLogJob     *jobs.CleanLogJob
	RuleDownloadJob *rulejob.RuleDownloadJob
	StatusReportJob *jobs.StatusReportJob
	Store           *fzservice.Store
}

// WireApp 根据配置和缓存装配 flow、工具和定时任务。
func WireApp(cfg *config.Config, store *fzservice.Store) *App {
	fzSvc := fzservice.NewFlowService(cfg, store)

	apiGroupApp := v1.ApiGroupApp
	serviceGroupApp := service.ServiceGroupApp
	serviceGroupApp.FZServiceGroup.FlowService = *fzSvc

	return &App{
		ApiGroupApp:     apiGroupApp,
		ServiceGroupApp: serviceGroupApp,
		CleanLogJob:     jobs.NewCleanLogJob(cfg.CleanLog),
		RuleDownloadJob: rulejob.NewRuleDownloadJob(cfg.SFTP, cfg.Device, &serviceGroupApp.FZServiceGroup.FlowService),
		StatusReportJob: jobs.NewStatusReportJob(cfg.StatusReport, &serviceGroupApp.FZServiceGroup.FlowService),
		Store:           store,
	}
}
