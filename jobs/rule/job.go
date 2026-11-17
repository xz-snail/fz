package rule

import (
	"context"
	"errors"
	"net/http"
	"time"

	"ctaf-fz/config"
	"ctaf-fz/global"
	"ctaf-fz/model/common"
	"ctaf-fz/model/common/response"
	fzmodel "ctaf-fz/model/fz"
	fzservice "ctaf-fz/service/fz"

	"go.uber.org/zap"
)

const (
	// ruleDownloadTimeoutBuffer 是单次下载在配置超时之外追加的缓冲时间。
	ruleDownloadTimeoutBuffer = 30 * time.Second
	// ruleFeedbackDelay 是下载成功后到执行反馈之间的等待时间。
	ruleFeedbackDelay = 5 * time.Second
	// maxRuleTaskFailures 任务失败次数上限，超过后删除任务。
	maxRuleTaskFailures = 5
)

// errRuleFeedbackFailed 表示反馈接口未返回成功状态（HTTP 非 200 或业务码非 200）。
var errRuleFeedbackFailed = errors.New("rule feedback failed")

// RuleDownloadJob 定时驱动全量规则下载链路，按任务标记执行对应步骤：
// todo_download 下载规则文件，todo_feedback 向上下游反馈；
// 任务反馈成功或任一步骤失败超过上限即整体删除。
type RuleDownloadJob struct {
	cfg        config.RuleDownloadConfig
	device     config.DeviceConfig
	downloader *Downloader
	service    *fzservice.FlowService
}

// NewRuleDownloadJob 创建规则下载定时任务。
func NewRuleDownloadJob(sftpCfg config.SFTPConfig, device config.DeviceConfig, service *fzservice.FlowService) *RuleDownloadJob {
	return &RuleDownloadJob{
		cfg:        sftpCfg.RuleDownload,
		device:     device,
		downloader: NewDownloader(sftpCfg),
		service:    service,
	}
}

// Start 启动规则下载定时任务，并立即执行一次检查。
func (j *RuleDownloadJob) Start(ctx context.Context) {
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

// run 读取下一条待处理任务，按当前标记分发到对应步骤。
func (j *RuleDownloadJob) run(ctx context.Context) {
	record, state, ok, err := j.service.NextRuleTask(ctx, j.device.SID)
	if err != nil {
		global.Logger.Warn("rule task read failed", zap.Error(err))
		return
	}
	if !ok {
		return
	}

	switch state {
	case fzservice.RuleTaskTodoDownload:
		j.download(ctx, record)
	case fzservice.RuleTaskTodoFeedback:
		j.feedback(ctx, record)
	}
}

// download 下载规则文件到本地，成功后标记为待反馈，并延迟触发反馈。
func (j *RuleDownloadJob) download(ctx context.Context, record fzmodel.RuleRecord) {
	// 下载超时在配置的连接超时之上追加缓冲，覆盖建连和传输收尾时间。
	downloadCtx, cancel := context.WithTimeout(ctx, j.downloader.cfg.Timeout+ruleDownloadTimeoutBuffer)
	defer cancel()

	result, err := j.downloader.Download(downloadCtx, record.Path)
	if err != nil {
		j.handleTaskFailure(ctx, record, "rule download(全量规则包下载)", downloadErrorMessage(err))
		return
	}
	// 标记 todo_feedback 并重置失败计数，反馈阶段可独立重试。
	if err := j.service.MarkRuleDownloaded(ctx, record.SID, record.Type, record.BathNo); err != nil {
		global.Logger.Warn("rule download state update failed(全量规则包下载)", zap.Error(err))
		return
	}
	global.Logger.Info("rule download succeeded(全量规则包下载)", zap.String("sid", record.SID), zap.String("bathNo", record.BathNo), zap.String("sourcePath", record.Path), zap.String("localPath", result.LocalPath))

	// 等待数秒再反馈；进程若在此期间退出，下一轮 job 会从 todo_feedback 继续。
	select {
	case <-ctx.Done():
		return
	case <-time.After(ruleFeedbackDelay):
		j.feedback(ctx, record)
	}
}

// handleTaskFailure 累计任务失败次数，超过上限则删除任务。
func (j *RuleDownloadJob) handleTaskFailure(ctx context.Context, record fzmodel.RuleRecord, action string, msg string) {
	failCount, err := j.service.IncrementRuleFailCount(ctx, record.SID, record.Type, record.BathNo)
	if err != nil {
		global.Logger.Warn("rule task fail count update failed", zap.Error(err))
		return
	}
	if failCount > maxRuleTaskFailures {
		if err := j.service.DeleteRuleTask(ctx, record.SID, record.Type, record.BathNo); err != nil {
			global.Logger.Warn("rule task delete failed", zap.Error(err))
			return
		}
		global.Logger.Warn("rule task failed too many times and deleted", zap.String("sid", record.SID), zap.String("bathNo", record.BathNo), zap.String("action", action), zap.Int("failCount", failCount), zap.String("msg", msg))
		return
	}
	global.Logger.Warn("rule task failed", zap.String("sid", record.SID), zap.String("bathNo", record.BathNo), zap.String("action", action), zap.Int("failCount", failCount), zap.String("msg", msg))
}

// feedback 将已下载的规则信息反馈下游并上报上游，两步都成功后才删除任务及下发记录。
func (j *RuleDownloadJob) feedback(ctx context.Context, record fzmodel.RuleRecord) {
	// 1. 反馈下游（全量规则下发）：将已下载的全量规则信息组装好转发给下游服务，失败按任务失败累计并等待下一轮重试。
	if err := j.forwardRuleUpgrade(ctx, record); err != nil {
		j.handleTaskFailure(ctx, record, "feedback-forwardRuleUpgrade(全量规则转发下游)", err.Error())
		return
	}
	// 2. 上报上游（全量规则消费反馈）：失败同样按任务失败累计并等待下一轮重试。
	if err := j.reportRuleUpgrade(ctx, record); err != nil {
		j.handleTaskFailure(ctx, record, "feedback-reportRuleUpgrade(全量规则消费反馈)", err.Error())
		return
	}
	// 3. 前两步都成功，任务完成，连同任务标记和下发记录一起删除。
	if err := j.service.DeleteRuleTask(ctx, record.SID, record.Type, record.BathNo); err != nil {
		global.Logger.Warn("rule task delete failed(全量规则数据删除)", zap.Error(err))
		return
	}
	global.Logger.Info("rule task completed and deleted(全量规则数据删除)", zap.String("sid", record.SID), zap.String("bathNo", record.BathNo), zap.String("type", record.Type))
}

// forwardRuleUpgrade 全量规则下发：将已下载的全量规则信息组装好转发给下游服务。
func (j *RuleDownloadJob) forwardRuleUpgrade(ctx context.Context, record fzmodel.RuleRecord) error {
	result, status := j.service.ForwardRuleUpgrade(ctx, record)
	if !ruleFeedbackSucceeded(status, result) {
		global.Logger.Warn("rule upgrade forward response failed (全量规则转发下游)", zap.Int("status", status), zap.Any("response", result))
		return errRuleFeedbackFailed
	}
	global.Logger.Info("rule upgrade forward succeeded (全量规则转发下游)", zap.String("sid", record.SID), zap.String("bathNo", record.BathNo), zap.Any("response", result))
	return nil
}

// reportRuleUpgrade 向上游发送全量规则消费反馈（1.5.4），失败不重试。
func (j *RuleDownloadJob) reportRuleUpgrade(ctx context.Context, record fzmodel.RuleRecord) error {
	report, status := j.service.UpgradeReport(ctx, record)
	if !ruleFeedbackSucceeded(status, report) {
		global.Logger.Warn("rule upgrade report response failed(全量规则消费反馈)", zap.Int("status", status), zap.Any("response", report))
		return errRuleFeedbackFailed
	}
	global.Logger.Info("rule upgrade report succeeded(全量规则消费反馈)", zap.String("sid", record.SID), zap.String("bathNo", record.BathNo), zap.Any("response", report))
	return nil
}

// ruleFeedbackSucceeded 判断反馈是否成功：HTTP 200 且业务码为 SUCCESS。
func ruleFeedbackSucceeded(status int, result map[string]any) bool {
	return status == http.StatusOK && common.NumericCode(result["code"]) == response.SUCCESS
}

// downloadErrorMessage 将下载错误转换为可读的中文提示。
func downloadErrorMessage(err error) string {
	if errors.Is(err, ErrInvalidURL) {
		return "下载链接有误"
	}
	if err != nil {
		return err.Error()
	}
	return "下载失败！"
}
