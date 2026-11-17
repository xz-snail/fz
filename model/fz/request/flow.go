package request

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"ctaf-fz/model/common"

	"github.com/gin-gonic/gin"
)

// FieldCheck 描述一个 legacy 请求字段的必填校验规则。
type FieldCheck struct {
	Field   string
	Message string
}

var (
	// RuleIncrementChecks 是增量规则接口的必填字段校验规则。
	RuleIncrementChecks = []FieldCheck{
		{Field: "timestamp", Message: "时间戳不能为空"},
		{Field: "sid", Message: "设备ID不能为空"},
		{Field: "type", Message: "设备业务类型不能为空"},
		{Field: "bathNo", Message: "特征库下发的批次号不能为空"},
	}
	// RuleUpgradeChecks 是全量规则下发接口的必填字段校验规则。
	RuleUpgradeChecks = []FieldCheck{
		{Field: "sid", Message: "设备ID不能为空"},
		{Field: "type", Message: "设备业务类型不能为空"},
		{Field: "timestamp", Message: "时间戳不能为空"},
		{Field: "ruleType", Message: "ruleType不能为空"},
		{Field: "version", Message: "平台规则库版本号不能为空"},
		{Field: "path", Message: "升级包FTP下载全路径不能为空"},
		{Field: "md5", Message: "升级包文件md5值不能为空"},
	}
	// UpgradeRedirectChecks 是导航规则接口的必填字段校验规则。
	UpgradeRedirectChecks = []FieldCheck{
		{Field: "sid", Message: "设备ID不能为空"},
		{Field: "type", Message: "设备业务类型不能为空"},
		{Field: "bathNo", Message: "特征库下发的批次号不能为空"},
	}
)

// ReadJSONMap 读取 Gin 请求体，并区分空 body 与非法 JSON。
func ReadJSONMap(c *gin.Context) (map[string]any, bool, error) {
	return ReadJSONMapWithLimit(c, 0)
}

func ReadJSONMapWithLimit(c *gin.Context, maxBodySizeBytes int64) (map[string]any, bool, error) {
	bodyReader := c.Request.Body
	if maxBodySizeBytes > 0 {
		bodyReader = http.MaxBytesReader(c.Writer, c.Request.Body, maxBodySizeBytes)
	}
	body, err := io.ReadAll(bodyReader)
	if err != nil {
		return nil, false, err
	}
	if len(bytes.TrimSpace(body)) == 0 {
		return nil, false, nil
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, false, err
	}
	return payload, true, nil
}

func IsBodyTooLarge(err error) bool {
	var maxBytesErr *http.MaxBytesError
	return errors.As(err, &maxBytesErr)
}

// MissingFieldMessage 返回第一个缺失必填字段对应的 legacy 错误文案。
func MissingFieldMessage(payload map[string]any, checks []FieldCheck) (string, bool) {
	for _, check := range checks {
		if common.StringValue(payload[check.Field]) == "" {
			return check.Message, true
		}
	}
	return "", false
}
