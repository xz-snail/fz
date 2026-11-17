package jobs

import (
	"context"
	"time"

	"ctaf-fz/config"
	"ctaf-fz/global"
	"ctaf-fz/model/common"
	"ctaf-fz/model/common/response"
	fzservice "ctaf-fz/service/fz"

	"go.uber.org/zap"
)

// StatusReportJob 定时向上游上报默认设备状态。
type StatusReportJob struct {
	cfg     config.StatusReportConfig
	service *fzservice.FlowService
}

// NewStatusReportJob 创建默认状态上报定时任务。
func NewStatusReportJob(cfg config.StatusReportConfig, service *fzservice.FlowService) *StatusReportJob {
	return &StatusReportJob{cfg: cfg, service: service}
}

// Start 启动状态上报定时任务，并立即执行一次上报。
func (j *StatusReportJob) Start(ctx context.Context) {
	if !j.cfg.Enabled || j.cfg.Interval <= 0 {
		return
	}
	go func() {
		j.run(ctx)
		ticker := time.NewTicker(j.cfg.Interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				j.run(ctx)
			}
		}
	}()
}

// run 上报一次默认设备状态，并记录结果日志。
func (j *StatusReportJob) run(ctx context.Context) {
	result, err := j.service.ForwardDefaultStatusReport(ctx)
	// fmt.Printf("status report job result: %+v, err: %v\n", result, err)
	if err != nil {
		global.Logger.Warn("status report job failed", zap.Error(err))
		return
	}
	if common.NumericCode(result["code"]) != response.SUCCESS {
		global.Logger.Warn("status report job response failed", zap.Any("response", result))
		return
	}
	global.Logger.Info("status report job succeeded", zap.Any("response", result))
}
