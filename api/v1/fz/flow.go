package fz

import (
	"net/http"
	"time"

	"ctaf-fz/global"
	"ctaf-fz/model/common"
	"ctaf-fz/model/common/response"
	fzRequest "ctaf-fz/model/fz/request"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// FlowApi 承载 FZ 兼容接口的 Gin 处理方法。
type FlowApi struct{}

// Health 返回服务健康状态。
func (h *FlowApi) Health(c *gin.Context) {
	response.ResultWithStatus(c, http.StatusOK, map[string]any{"code": response.SUCCESS, "msg": "ok"})
}

func (h *FlowApi) readJSONMap(c *gin.Context, api string) (map[string]any, bool, error) {
	maxBodySizeBytes := int64(0)
	if global.Config != nil {
		maxBodySizeBytes = global.Config.Server.MaxBodySizeBytes
	}
	payload, hasBody, err := fzRequest.ReadJSONMapWithLimit(c, maxBodySizeBytes)
	if err == nil {
		global.Logger.Info("api request received", zap.String("api", api), zap.Any("payload", payload))
		return payload, hasBody, nil
	}
	if fzRequest.IsBodyTooLarge(err) {
		h.failInvalid(c, api, "请求体过大", payload)
		return nil, false, err
	}
	h.failInvalid(c, api, "请求参数不能为空", payload)
	return nil, false, err
}

func (h *FlowApi) failInvalid(c *gin.Context, api, message string, payload map[string]any) {
	global.Logger.Error("api invalid request", zap.String("api", api), zap.String("message", message), zap.Any("payload", payload))
	response.Fail(c, message)
}

// StatusReport 1.5.1 处理设备状态信息同步接口说明（心跳接口）
func (h *FlowApi) StatusReport(c *gin.Context) {
	api := "statusreport"
	payload, hasBody, err := h.readJSONMap(c, api)
	if err != nil {
		return
	}
	if !hasBody {
		h.failInvalid(c, api, "请求参数不能为空", payload)
		return
	}
	status, _ := payload["status"].(map[string]any)
	if status == nil || common.StringValue(status["sid"]) == "" {
		h.failInvalid(c, api, "请求sid参数不能为空", payload)
		return
	}
	status["timestamp"] = time.Now().Unix()
	response.Ok(c)
}

// RuleIncrement 1.5.2 校验并转发增量规则下发请求。
func (h *FlowApi) RuleIncrement(c *gin.Context) {
	api := "rule/increment"
	payload, hasBody, err := h.readJSONMap(c, api)
	if err != nil {
		return
	}
	if !hasBody {
		h.failInvalid(c, api, "请求参数不能为空", payload)
		return
	}
	if message, ok := fzRequest.MissingFieldMessage(payload, fzRequest.RuleIncrementChecks); ok {
		h.failInvalid(c, api, message, payload)
		return
	}
	result, err := flowService.ForwardRuleIncrement(c.Request.Context(), payload)
	// fmt.Printf("rule increment result: %+v\n", result)
	if err == nil && common.NumericCode(result["code"]) == response.SUCCESS {
		response.Ok(c)
		return
	}
	global.Logger.Error("api forward failed", zap.String("api", api), zap.Any("payload", payload), zap.Any("response", result), zap.Error(err))
	response.Result(c, response.ERROR, "失败", true)
}

// RuleUpgrade  1.5.3 保存全量规则下发信息
func (h *FlowApi) RuleUpgrade(c *gin.Context) {
	api := "rule/upgrade"
	payload, hasBody, err := h.readJSONMap(c, api)
	if err != nil {
		return
	}
	if !hasBody {
		h.failInvalid(c, api, "请求参数不能为空", payload)
		return
	}
	if message, ok := fzRequest.MissingFieldMessage(payload, fzRequest.RuleUpgradeChecks); ok {
		h.failInvalid(c, api, message, payload)
		return
	}
	bathNo := c.GetHeader("bathNo")
	if bathNo == "" {
		h.failInvalid(c, api, "特征库下发的批次号不能为空", payload)
		return
	}
	if err := flowService.SaveRuleRecord(c.Request.Context(), payload, bathNo); err != nil {
		global.Logger.Error("api save rule record failed", zap.String("api", api), zap.Any("payload", payload), zap.Error(err))
		response.Fail(c, "失败")
		return
	}
	response.Ok(c)
}

// UpgradeRedirect 1.5.5重定向规则下发接口 。
func (h *FlowApi) UpgradeRedirect(c *gin.Context) {
	api := "upgrade/redirect"
	payload, hasBody, err := h.readJSONMap(c, api)
	if err != nil {
		return
	}
	if !hasBody {
		h.failInvalid(c, api, "请求参数不能为空", payload)
		return
	}
	if message, ok := fzRequest.MissingFieldMessage(payload, fzRequest.UpgradeRedirectChecks); ok {
		h.failInvalid(c, api, message, payload)
		return
	}
	if _, ok := payload["isEnable"]; !ok {
		h.failInvalid(c, api, "是否启用导航规则不能为空", payload)
		return
	}
	if common.NumericCode(payload["isEnable"]) == 1 && common.StringValue(payload["redirectUrl"]) == "" {
		h.failInvalid(c, api, "重定向/导航URL地址不能为空", payload)
		return
	}
	result, err := flowService.UpgradeRedirect(c.Request.Context(), payload)
	if err != nil {
		global.Logger.Error("api forward failed", zap.String("api", api), zap.Any("payload", payload), zap.Error(err))
		response.Fail(c, "失败")
		return
	}
	response.ResultWithStatus(c, http.StatusOK, result)
}
