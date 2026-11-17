package fz

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"ctaf-fz/config"
	"ctaf-fz/model/common"
	fzmodel "ctaf-fz/model/fz"
)

const (
	// RuleTaskTodoDownload 表示规则待下载（todo_download）：保存下发记录后的标记。
	RuleTaskTodoDownload = "todo_download"
	// RuleTaskTodoFeedback 表示规则已下载到本地、待反馈（todo_feedback）。
	RuleTaskTodoFeedback = "todo_feedback"
)

// Store 使用内存缓存保存规则下载信息和下载任务标记。
type Store struct {
	mu      sync.RWMutex
	records map[string]fzmodel.RuleRecord
	tasks   map[string]string
}

// NewStore 创建规则状态缓存实例。
func NewStore() *Store {
	return &Store{
		records: make(map[string]fzmodel.RuleRecord),
		tasks:   make(map[string]string),
	}
}

// Close 兼容服务退出流程，内存缓存无需释放外部资源。
func (s *Store) Close() error {
	return nil
}

// SaveTypedRuleRecord 按业务类型和批次号保存规则下发记录。
func (s *Store) SaveTypedRuleRecord(ctx context.Context, record fzmodel.RuleRecord, typ string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records[ruleTaskKey(record.SID, typ, record.BathNo)] = record
	return nil
}

// SetTypedRuleTask 按 sid、业务类型和批次号更新下载任务标记。
func (s *Store) SetTypedRuleTask(ctx context.Context, sid, typ, bathNo, state string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tasks[ruleTaskKey(sid, typ, bathNo)] = state
	return nil
}

// IncrementRuleFailCount 累计任务失败次数并返回最新值。
func (s *Store) IncrementRuleFailCount(ctx context.Context, sid, typ, bathNo string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := ruleTaskKey(sid, typ, bathNo)
	record := s.records[key]
	record.FailCount++
	record.UpdatedAt = time.Now()
	s.records[key] = record
	return record.FailCount, nil
}

// ResetRuleFailCount 重置任务失败计数，进入待反馈阶段时调用。
func (s *Store) ResetRuleFailCount(ctx context.Context, sid, typ, bathNo string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := ruleTaskKey(sid, typ, bathNo)
	record := s.records[key]
	record.FailCount = 0
	record.UpdatedAt = time.Now()
	s.records[key] = record
	return nil
}

// DeleteTypedRuleTask 删除任务标记及对应的规则下发记录，反馈成功或失败超过上限时调用。
func (s *Store) DeleteTypedRuleTask(ctx context.Context, sid, typ, bathNo string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := ruleTaskKey(sid, typ, bathNo)
	delete(s.tasks, key)
	delete(s.records, key)
	return nil
}

// NextRuleTask 按创建时间获取下一条待处理任务；缓存中的任务均为待下载或待反馈，
// 反馈成功或失败超限的任务已被删除。
func (s *Store) NextRuleTask(ctx context.Context, sid string) (fzmodel.RuleRecord, string, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var next fzmodel.RuleRecord
	var nextState string
	for key, state := range s.tasks {
		record, ok := s.records[key]
		if !ok || record.SID != sid {
			continue
		}
		if next.BathNo == "" || record.CreatedAt.Before(next.CreatedAt) {
			next = record
			nextState = state
		}
	}
	if next.BathNo == "" {
		return fzmodel.RuleRecord{}, "", false, nil
	}
	return next, nextState, true, nil
}

// FlowService 封装 FZ 规则下发、状态和上报的业务逻辑。
type FlowService struct {
	cfg    *config.Config
	store  *Store
	client *http.Client
}

// NewFlowService 创建 FZ flow 业务服务。
func NewFlowService(cfg *config.Config, store *Store) *FlowService {
	return &FlowService{
		cfg:    cfg,
		store:  store,
		client: &http.Client{Timeout: cfg.RemoteAPIs.Timeout},
	}
}

// NextRuleTask 从缓存中读取下一条规则下载任务。
func (s *FlowService) NextRuleTask(ctx context.Context, sid string) (fzmodel.RuleRecord, string, bool, error) {
	return s.store.NextRuleTask(ctx, sid)
}

// MarkRuleDownloaded 标记规则已下载、待反馈（todo_feedback），并重置该阶段失败计数。
func (s *FlowService) MarkRuleDownloaded(ctx context.Context, sid, typ, bathNo string) error {
	if err := s.store.SetTypedRuleTask(ctx, sid, typ, bathNo, RuleTaskTodoFeedback); err != nil {
		return err
	}
	return s.store.ResetRuleFailCount(ctx, sid, typ, bathNo)
}

func (s *FlowService) IncrementRuleFailCount(ctx context.Context, sid, typ, bathNo string) (int, error) {
	return s.store.IncrementRuleFailCount(ctx, sid, typ, bathNo)
}

// DeleteRuleTask 删除规则任务标记及下发记录，反馈成功或失败超过上限时调用。
func (s *FlowService) DeleteRuleTask(ctx context.Context, sid, typ, bathNo string) error {
	return s.store.DeleteTypedRuleTask(ctx, sid, typ, bathNo)
}

// ForwardDefaultStatusReport 上报心跳到上游。
func (s *FlowService) ForwardDefaultStatusReport(ctx context.Context) (map[string]any, error) {
	return s.postJSON(ctx, s.cfg.RemoteAPIs.StatusReportURL, s.defaultStatusReportPayload(), nil)
}

func (s *FlowService) defaultStatusReportPayload() map[string]any {
	return map[string]any{
		"status": map[string]any{
			"sid":       s.cfg.Device.SID,
			"type":      "5",
			"timestamp": time.Now().Unix(),
			"vendorid":  "eversec",
			"config": map[string]any{
				"stfreq":  "100",
				"hbfreq":  "5",
				"devtype": "5",
				"softver": "http",
				"vender":  "eversec",
				"devip":   "200.200.200.2",
			},
			"db": []map[string]any{
				{
					"id":      "102323",
					"version": "1.0",
					"num":     "100",
				},
			},
			"sysinfor": map[string]any{
				"CPU": []map[string]any{
					{"id": "0", "text": "0.42"},
					{"id": "1", "text": "0.45"},
					{"id": "2", "text": "0.49"},
					{"id": "3", "text": "0.47"},
					{"id": "4", "text": "0.44"},
				},
				"MEM":   "0.97",
				"HDISK": "0.06",
				"IF_THRPUT": []map[string]any{
					{"name": "lo", "text": "0.00"},
				},
			},
		},
	}
}

// ForwardRuleIncrement 转发增量规则下发请求到下游服务。
func (s *FlowService) ForwardRuleIncrement(ctx context.Context, payload map[string]any) (map[string]any, error) {
	out := map[string]any{
		"sid":       payload["sid"],
		"type":      payload["type"],
		"ruleList":  payload["ruleList"],
		"timestamp": time.Now().Unix(),
		"remarks":   payload["remarks"],
		"bathNo":    payload["bathNo"],
	}
	return s.postJSON(ctx, s.cfg.RemoteAPIs.RuleIncrementURL, out, nil)
}

// SaveRuleRecord 将全量规则下发信息保存到缓存。 1.5.3全量规则下发接口说明
func (s *FlowService) SaveRuleRecord(ctx context.Context, payload map[string]any, bathNo string) error {
	now := time.Now()
	// 从下发请求中提取业务字段，组装规则下发记录（原始 payload 一并留存）。
	record := fzmodel.RuleRecord{
		SID:       common.StringValue(payload["sid"]),
		Type:      common.StringValue(payload["type"]),
		BathNo:    bathNo,
		RuleType:  common.StringValue(payload["ruleType"]),
		Version:   common.StringValue(payload["version"]),
		Path:      common.StringValue(payload["path"]),
		MD5:       common.StringValue(payload["md5"]),
		Raw:       payload,
		CreatedAt: now,
		UpdatedAt: now,
	}
	// 记录按 sid_type_bathNo 维度覆盖保存；同批次号重复下发会重置记录的处理状态。
	if err := s.store.SaveTypedRuleRecord(ctx, record, record.Type); err != nil {
		return err
	}
	// 初始任务标记为 todo_download（待下载），RuleDownloadJob 下一轮调度时执行下载。
	return s.store.SetTypedRuleTask(ctx, record.SID, record.Type, record.BathNo, RuleTaskTodoDownload)
}

// ForwardRuleUpgrade 将已下载的全量规则信息转发给下游服务。
func (s *FlowService) ForwardRuleUpgrade(ctx context.Context, record fzmodel.RuleRecord) (map[string]any, int) {
	forwardPath := joinURL(s.cfg.SFTP.DownloadURLPrefix, filepath.Base(record.Path))
	payload := map[string]any{
		"sid":       record.SID,
		"type":      record.Type,
		"timestamp": time.Now().Unix(),
		"ruleType":  record.RuleType,
		"version":   record.Version,
		"path":      forwardPath,
		"md5":       record.MD5,
	}
	response, err := s.postJSON(ctx, s.cfg.RemoteAPIs.RuleUpgradeURL, payload, map[string]string{"bathNo": record.BathNo})
	if err != nil {
		return map[string]any{"code": 400, "msg": err.Error(), "data": nil}, http.StatusOK
	}
	return response, http.StatusOK
}

// UpgradeReport 上报缓存中已下载规则的升级结果。1.5.4全量规则消费反馈接口
func (s *FlowService) UpgradeReport(ctx context.Context, record fzmodel.RuleRecord) (map[string]any, int) {
	payload := map[string]any{
		"response": map[string]any{
			"sid":       record.SID,
			"type":      record.Type,
			"timestamp": time.Now().Unix(),
			"db": map[string]any{
				"fileType": record.RuleType,
				"version":  "1.1.0",
				"md5":      record.MD5,
				"code":     0,
				"text":     "rule update success",
			},
		},
	}
	response, err := s.postJSON(ctx, s.cfg.RemoteAPIs.UpgradeReportURL, payload, map[string]string{"bathNo": record.BathNo})
	if err != nil {
		return map[string]any{"code": 400, "msg": err.Error(), "data": nil}, http.StatusOK
	}
	return response, http.StatusOK
}

// UpgradeRedirect  1.5.5重定向规则下发接口
func (s *FlowService) UpgradeRedirect(ctx context.Context, payload map[string]any) (map[string]any, error) {
	payload["timestamp"] = time.Now().Unix()
	return s.postJSON(ctx, s.cfg.RemoteAPIs.UpgradeRedirectURL, payload, nil)
}

func (s *FlowService) postJSON(ctx context.Context, url string, payload any, headers map[string]string) (map[string]any, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json;charset=utf-8")
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response (status %d): %w", resp.StatusCode, err)
	}
	var result map[string]any
	if err := json.Unmarshal(respBody, &result); err != nil {
		snippet := string(respBody)
		if len(snippet) > 200 {
			snippet = snippet[:200]
		}
		return nil, fmt.Errorf("non-json response (status %d): %s", resp.StatusCode, snippet)
	}
	return result, nil
}

func joinURL(base, path string) string {
	return strings.TrimRight(base, "/") + "/" + strings.TrimLeft(path, "/")
}

func ruleKey(sid, typ string) string {
	return sid + "_" + typ + "_sidlog"
}

func ruleTaskKey(sid, typ, bathNo string) string {
	return ruleKey(sid, typ) + "_" + bathNo
}
