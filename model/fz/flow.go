package fz

import "time"

// RuleRecord 保存一次规则下发的业务字段和处理状态。
type RuleRecord struct {
	SID       string         `json:"sid"`
	Type      string         `json:"type"`
	BathNo    string         `json:"bathNo"`
	RuleType  string         `json:"ruleType"`
	Version   string         `json:"version"`
	Path      string         `json:"path"`
	MD5       string         `json:"md5"`
	FailCount int            `json:"failCount"`
	Raw       map[string]any `json:"raw"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
}
