package controller

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel/novelai"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

// RelayNovelAINative 处理 NovelAI 原生生图接口 (POST /ai/generate-image)
// 适配 st-chatu8 / NovelAI 官方前端等客户端直连
func RelayNovelAINative(c *gin.Context) {
	startTime := time.Now()
	userId := c.GetInt("id")
	if userId <= 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read request body"})
		return
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(bodyBytes))

	var rawReq struct {
		Input      string         `json:"input"`
		Model      string         `json:"model"`
		Action     string         `json:"action"`
		Parameters map[string]any `json:"parameters"`
	}
	if err := json.Unmarshal(bodyBytes, &rawReq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON payload: " + err.Error()})
		return
	}

	modelName := strings.TrimSpace(rawReq.Model)
	if modelName == "" {
		modelName = "nai-diffusion-4-5-full"
	}

	width := 832
	height := 1216
	steps := 28
	n := 1
	if rawReq.Parameters != nil {
		if w, ok := rawReq.Parameters["width"].(float64); ok && w > 0 {
			width = int(w)
		}
		if h, ok := rawReq.Parameters["height"].(float64); ok && h > 0 {
			height = int(h)
		}
		if s, ok := rawReq.Parameters["steps"].(float64); ok && s > 0 {
			steps = int(s)
		}
		if ns, ok := rawReq.Parameters["n_samples"].(float64); ok && ns > 0 {
			n = int(ns)
		} else if ns, ok := rawReq.Parameters["n"].(float64); ok && ns > 0 {
			n = int(ns)
		}
	}

	spec := service.NovelAIParamSpec{
		Model:  modelName,
		Width:  width,
		Height: height,
		Steps:  steps,
		N:      n,
		Action: rawReq.Action,
	}

	// 1. 精准判定：免费图 / 电量生图 / Anlas 付费图 / 复合图
	billingResult := service.ClassifyNovelAIRequest(spec)

	// 2. 前置拦截检查：若余额为负且非免费图，立即驳回
	if apiErr := service.PreCheckNovelAIBilling(userId, billingResult); apiErr != nil {
		c.JSON(apiErr.StatusCode, gin.H{
			"error":   apiErr.Error(),
			"message": apiErr.Error(),
		})
		return
	}

	// 3. 渠道选择与负载均衡：寻找可用 NovelAI 渠道
	userGroup := c.GetString("group")
	if userGroup == "" {
		userGroup = "default"
	}

	channel, err := model.GetRandomSatisfiedChannel(userGroup, modelName, 0, nil)
	if err != nil || channel == nil {
		// 备用兜底：直接按 ChannelTypeNovelAI 查找可用渠道
		var channels []*model.Channel
		model.DB.Where("type = ? AND status = ?", constant.ChannelTypeNovelAI, common.ChannelStatusEnabled).Find(&channels)
		if len(channels) > 0 {
			channel = channels[common.GetRandomInt(len(channels))]
		}
	}

	if channel == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "No available NovelAI channel"})
		return
	}

	// 4. 单并发排队锁 + 0.5s~3.0s 防封随机延迟
	releaseLock, err := novelai.AcquireChannelLock(c.Request.Context(), channel.Id)
	if err != nil {
		c.JSON(http.StatusRequestTimeout, gin.H{"error": "Channel queue timeout: " + err.Error()})
		return
	}
	defer releaseLock()

	// 5. 向上游发起请求并清洗设备指纹
	baseUrl := strings.TrimRight(channel.GetBaseURL(), "/")
	if baseUrl == "" {
		baseUrl = "https://image.novelai.net"
	}
	targetURL := fmt.Sprintf("%s/ai/generate-image", baseUrl)
	logger.LogInfo(c, fmt.Sprintf("[NovelAI Handler] Selected channel ID=%d, Name=%s, TargetURL=%s", channel.Id, channel.Name, targetURL))

	// 5.1 官方真实消耗感知：生图前通过 GET /user/subscription 查询账号实时 Anlas 和 电量(usage)
	balBefore, errBefore := service.FetchOfficialSubscriptionStatus(c.Request.Context(), baseUrl, channel.Key)
	if errBefore == nil {
		logger.LogInfo(c, fmt.Sprintf("[NovelAI Native] 官方账号生图前剩余: Anlas=%d, 电量(Power/Usage)=%d", balBefore.Anlas, balBefore.Power))
	} else {
		logger.LogWarn(c, fmt.Sprintf("[NovelAI Native] 查询官方生图前余额失败: %v", errBefore))
	}

	upstreamReq, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPost, targetURL, bytes.NewReader(bodyBytes))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create upstream request: " + err.Error()})
		return
	}

	// 彻底清洗下游客户端 IP/UA 并伪装服务器统一指纹
	novelai.SanitizeAndSpoofHeaders(upstreamReq, channel.Key)

	client := &http.Client{
		Timeout: 300 * time.Second,
	}
	upstreamResp, err := client.Do(upstreamReq)
	if err != nil {
		logger.LogError(c, fmt.Sprintf("NovelAI request failed: %v", err))
		c.JSON(http.StatusBadGateway, gin.H{"error": "Upstream NovelAI error: " + err.Error()})
		return
	}
	defer upstreamResp.Body.Close()

	respBytes, err := io.ReadAll(upstreamResp.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read upstream response: " + err.Error()})
		return
	}

	if upstreamResp.StatusCode != http.StatusOK {
		logger.LogError(c, fmt.Sprintf("NovelAI error response (%d): %s", upstreamResp.StatusCode, string(respBytes)))
		c.Data(upstreamResp.StatusCode, upstreamResp.Header.Get("Content-Type"), respBytes)
		return
	}

	// 6. 真实查询官方生图后余额并计算差值 (通过 GET /user/subscription 获取实际 Anlas 和电量消耗)
	if errBefore == nil {
		balAfter, errAfter := service.FetchOfficialSubscriptionStatus(c.Request.Context(), baseUrl, channel.Key)
		if errAfter == nil {
			logger.LogInfo(c, fmt.Sprintf("[NovelAI Native] 官方账号生图后剩余: Anlas=%d, 电量=%d", balAfter.Anlas, balAfter.Power))
			
			// 动态计算 Anlas 消耗
			anlasConsumed := balBefore.Anlas - balAfter.Anlas
			if anlasConsumed < 0 {
				anlasConsumed = 0
			}
			billingResult.AnlasCost = anlasConsumed

			// 动态计算 电量(Power) 消耗 (基于 usage.percent 差值)
			powerConsumed := balBefore.Power - balAfter.Power
			if powerConsumed < 0 {
				powerConsumed = 0
			}
			billingResult.PowerCost = powerConsumed

			logger.LogInfo(c, fmt.Sprintf("[NovelAI Native] 官方实际扣除: Anlas=%d (前:%d -> 后:%d), 电量(Power)=%d (前:%d -> 后:%d)",
				anlasConsumed, balBefore.Anlas, balAfter.Anlas,
				powerConsumed, balBefore.Power, balAfter.Power))

			if billingResult.PowerCost > 0 && billingResult.AnlasCost > 0 {
				billingResult.Type = service.BillingTypeComposite
			} else if billingResult.PowerCost > 0 {
				billingResult.Type = service.BillingTypePower
			} else if billingResult.AnlasCost > 0 {
				billingResult.Type = service.BillingTypeAnlas
			} else {
				billingResult.Type = service.BillingTypeFree
			}
		} else {
			logger.LogWarn(c, fmt.Sprintf("[NovelAI Native] 查询官方生图后余额/用量失败: %v, 回退默认规则", errAfter))
		}
	}

	// 7. 后置实际扣减余额 (基于官方真实扣减数扣除用户余额)
	if settleErr := service.SettleNovelAIBilling(userId, billingResult); settleErr != nil {
		logger.LogError(c, fmt.Sprintf("SettleNovelAIBilling failed for user %d: %v", userId, settleErr))
	} else {
		logger.LogInfo(c, fmt.Sprintf("NovelAI generation completed for user %d, result: %s", userId, billingResult.String()))
		tokenId := c.GetInt("token_id")
		tokenName := c.GetString("token_name")
		logOther := model.NewLogOther()
		logOther.SetPublic("power_cost", billingResult.PowerCost)
		logOther.SetPublic("anlas_cost", billingResult.AnlasCost)
		logOther.SetPublic("novelai", true)

		model.RecordConsumeLog(c, userId, model.RecordConsumeLogParams{
			ChannelId:        channel.Id,
			PromptTokens:     1,
			CompletionTokens: 1,
			ModelName:        modelName,
			TokenName:        tokenName,
			Quota:            billingResult.AnlasCost,
			Content:          billingResult.LogString(),
			TokenId:          tokenId,
			UseTimeSeconds:   int(time.Since(startTime).Seconds()),
			IsStream:         false,
			Group:            userGroup,
			Other:            logOther,
		})
	}

	// 7. 原汁原味透传回客户端 (Zip 流或 PNG)
	contentType := upstreamResp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/x-zip-compressed"
	}
	c.Data(http.StatusOK, contentType, respBytes)
}
