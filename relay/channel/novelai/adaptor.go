package novelai

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

type Adaptor struct {
	ReleaseLock func()
}

func (a *Adaptor) Init(info *relaycommon.RelayInfo) {
}

func (a *Adaptor) GetChannelName() string {
	return "NovelAI"
}

func (a *Adaptor) GetModelList() []string {
	return []string{
		"nai-diffusion-4-5-full",
		"nai-diffusion-4-5-curated",
		"nai-diffusion-4-full",
		"nai-diffusion-4-curated-preview",
		"nai-diffusion-3",
		"nai-diffusion-furry-3",
		"safe-diffusion",
		"nai-diffusion",
	}
}

func (a *Adaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	baseUrl := strings.TrimRight(info.ChannelBaseUrl, "/")
	if baseUrl == "" {
		baseUrl = "https://image.novelai.net"
	}
	return fmt.Sprintf("%s/ai/generate-image", baseUrl), nil
}

func (a *Adaptor) SetupRequestHeader(c *gin.Context, req *http.Header, info *relaycommon.RelayInfo) error {
	return nil
}

// NovelAIGeneratePayload 对应官方 API Payload 格式
type NovelAIGeneratePayload struct {
	Input             string                 `json:"input"`
	Model             string                 `json:"model"`
	Action            string                 `json:"action"`
	Parameters        map[string]any         `json:"parameters"`
	UseNewSharedTrial bool                   `json:"use_new_shared_trial,omitempty"`
}

func parseSize(sizeStr string) (int, int) {
	parts := strings.Split(strings.ToLower(sizeStr), "x")
	if len(parts) == 2 {
		w, _ := strconv.Atoi(parts[0])
		h, _ := strconv.Atoi(parts[1])
		if w > 0 && h > 0 {
			return w, h
		}
	}
	return 832, 1216
}

func (a *Adaptor) ConvertImageRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.ImageRequest) (any, error) {
	modelName := info.OriginModelName
	if modelName == "" {
		modelName = request.Model
	}
	if modelName == "" {
		modelName = "nai-diffusion-4-5-full"
	}

	width, height := parseSize(request.Size)
	steps := 28
	n := 1
	if request.N != nil && *request.N > 0 {
		n = int(*request.N)
	}

	spec := service.NovelAIParamSpec{
		Model:  modelName,
		Width:  width,
		Height: height,
		Steps:  steps,
		N:      n,
		Action: "generate",
	}

	billingResult := service.ClassifyNovelAIRequest(spec)
	c.Set("novelai_billing_result", billingResult)
	c.Set("novelai_billing_type", billingResult.Type)
	c.Set("novelai_cost", billingResult.PowerCost+billingResult.AnlasCost)

	if apiErr := service.PreCheckNovelAIBilling(info.UserId, billingResult); apiErr != nil {
		return nil, apiErr
	}

	params := map[string]any{
		"width":          width,
		"height":         height,
		"scale":          6.0,
		"sampler":        "k_euler",
		"steps":          steps,
		"n_samples":      n,
		"uc":             "lowres, {bad}, error, fewer, extra, missing, worst quality",
		"sm":             false,
		"sm_dyn":         false,
		"dynamic_thresholding": false,
	}

	if len(request.ExtraFields) > 0 {
		var extra map[string]any
		if err := json.Unmarshal(request.ExtraFields, &extra); err == nil {
			for k, v := range extra {
				params[k] = v
			}
		}
	}

	payload := NovelAIGeneratePayload{
		Input:      request.Prompt,
		Model:      modelName,
		Action:     "generate",
		Parameters: params,
	}
	if strings.Contains(modelName, "nai-diffusion-5") {
		payload.UseNewSharedTrial = true
	}

	return payload, nil
}

func (a *Adaptor) ConvertOpenAIRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest) (any, error) {
	if request == nil {
		return nil, errors.New("request is nil")
	}

	prompt := ""
	if len(request.Messages) > 0 {
		prompt = request.Messages[len(request.Messages)-1].StringContent()
	} else if request.Prompt != nil {
		prompt = fmt.Sprintf("%v", request.Prompt)
	}

	modelName := info.OriginModelName
	if modelName == "" {
		modelName = request.Model
	}
	if modelName == "" {
		modelName = "nai-diffusion-4-5-full"
	}

	width := 832
	height := 1216
	steps := 28
	n := 1

	spec := service.NovelAIParamSpec{
		Model:  modelName,
		Width:  width,
		Height: height,
		Steps:  steps,
		N:      n,
		Action: "generate",
	}

	billingResult := service.ClassifyNovelAIRequest(spec)
	c.Set("novelai_billing_result", billingResult)
	c.Set("novelai_billing_type", billingResult.Type)
	c.Set("novelai_cost", billingResult.PowerCost+billingResult.AnlasCost)
	c.Set("novelai_chat_prompt", prompt)

	if apiErr := service.PreCheckNovelAIBilling(info.UserId, billingResult); apiErr != nil {
		return nil, apiErr
	}

	params := map[string]any{
		"width":     width,
		"height":    height,
		"scale":     6.0,
		"sampler":   "k_euler",
		"steps":     steps,
		"n_samples": n,
		"uc":        "lowres, {bad}, error, fewer, extra, missing, worst quality",
	}

	payload := NovelAIGeneratePayload{
		Input:      prompt,
		Model:      modelName,
		Action:     "generate",
		Parameters: params,
	}
	if strings.Contains(modelName, "nai-diffusion-5") {
		payload.UseNewSharedTrial = true
	}

	return payload, nil
}

func (a *Adaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error) {
	releaseLock, err := AcquireChannelLock(c.Request.Context(), info.ChannelId)
	if err != nil {
		return nil, types.NewErrorWithStatusCode(err, types.ErrorCodeDoRequestFailed, http.StatusRequestTimeout)
	}
	a.ReleaseLock = releaseLock

	targetURL, err := a.GetRequestURL(info)
	if err != nil {
		if a.ReleaseLock != nil {
			a.ReleaseLock()
		}
		return nil, err
	}

	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPost, targetURL, requestBody)
	if err != nil {
		if a.ReleaseLock != nil {
			a.ReleaseLock()
		}
		return nil, err
	}

	// 执行设备指纹清洗并伪装服务端指纹
	SanitizeAndSpoofHeaders(req, info.ApiKey)

	// 官方真实消耗感知：生图前通过 GET /user/subscription 查询账号实时 Anlas 和 电量(usage)
	balBefore, errBefore := service.FetchOfficialSubscriptionStatus(c.Request.Context(), info.ChannelBaseUrl, info.ApiKey)
	if errBefore == nil {
		c.Set("novelai_bal_before", balBefore)
		c.Set("novelai_bal_before_ok", true)
	}

	client := &http.Client{
		Timeout: 300 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		if a.ReleaseLock != nil {
			a.ReleaseLock()
		}
		return nil, err
	}
	return resp, nil
}

// ExtractImagesFromNovelAIResponse 从 NovelAI 响应中解析出 PNG 图像的 base64 列表
func ExtractImagesFromNovelAIResponse(respBytes []byte) ([]string, error) {
	if len(respBytes) == 0 {
		return nil, errors.New("empty response body from NovelAI")
	}

	// 1. 尝试作为 Zip 文件解压提取 PNG
	zipReader, err := zip.NewReader(bytes.NewReader(respBytes), int64(len(respBytes)))
	if err == nil {
		var images []string
		for _, f := range zipReader.File {
			if strings.HasSuffix(strings.ToLower(f.Name), ".png") || strings.HasSuffix(strings.ToLower(f.Name), ".jpg") {
				rc, err := f.Open()
				if err != nil {
					continue
				}
				buf, err := io.ReadAll(rc)
				rc.Close()
				if err == nil && len(buf) > 0 {
					images = append(images, base64.StdEncoding.EncodeToString(buf))
				}
			}
		}
		if len(images) > 0 {
			return images, nil
		}
	}

	// 2. 检查是否为直接输出的 PNG 二进制流
	if bytes.HasPrefix(respBytes, []byte("\x89PNG\r\n\x1a\n")) {
		return []string{base64.StdEncoding.EncodeToString(respBytes)}, nil
	}

	// 3. 尝试作为 JSON 格式解析
	var jsonResp struct {
		Images []string `json:"images"`
		Data   string   `json:"data"`
		Error  string   `json:"error"`
		Message string  `json:"message"`
	}
	if err := json.Unmarshal(respBytes, &jsonResp); err == nil {
		if jsonResp.Error != "" || jsonResp.Message != "" {
			msg := jsonResp.Error
			if msg == "" {
				msg = jsonResp.Message
			}
			return nil, errors.New(msg)
		}
		if len(jsonResp.Images) > 0 {
			return jsonResp.Images, nil
		}
		if jsonResp.Data != "" {
			return []string{jsonResp.Data}, nil
		}
	}

	return nil, errors.New("failed to parse image data from NovelAI response")
}

func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (usage any, err *types.NewAPIError) {
	defer func() {
		if a.ReleaseLock != nil {
			a.ReleaseLock()
		}
	}()

	if resp == nil {
		return nil, types.NewError(errors.New("upstream response is nil"), types.ErrorCodeBadResponse)
	}
	defer service.CloseResponseBodyGracefully(resp)

	bodyBytes, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return nil, types.NewOpenAIError(readErr, types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError)
	}

	if resp.StatusCode != http.StatusOK {
		logger.LogError(c, fmt.Sprintf("NovelAI upstream error status %d: %s", resp.StatusCode, string(bodyBytes)))
		return nil, types.WithOpenAIError(types.OpenAIError{
			Message: fmt.Sprintf("NovelAI error: %s", string(bodyBytes)),
			Type:    "novelai_error",
			Code:    strconv.Itoa(resp.StatusCode),
		}, resp.StatusCode)
	}

	images, parseErr := ExtractImagesFromNovelAIResponse(bodyBytes)
	if parseErr != nil {
		logger.LogError(c, fmt.Sprintf("Extract image error: %v", parseErr))
		return nil, types.NewOpenAIError(parseErr, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}

	// 后置扣费结算 (基于官方 GET /user/subscription 真实扣减)
	if billingAny, exists := c.Get("novelai_billing_result"); exists {
		if billingResult, ok := billingAny.(service.NovelAIBillingResult); ok {
			if beforeOk, okOk := c.Get("novelai_bal_before_ok"); okOk && beforeOk.(bool) {
				balBeforeVal, hasBal := c.Get("novelai_bal_before")
				if hasBal {
					if balBefore, okCast := balBeforeVal.(service.NovelAIBalanceStatus); okCast {
						balAfter, errAfter := service.FetchOfficialSubscriptionStatus(c.Request.Context(), info.ChannelBaseUrl, info.ApiKey)
						if errAfter == nil {
							anlasConsumed := balBefore.Anlas - balAfter.Anlas
							if anlasConsumed < 0 {
								anlasConsumed = 0
							}
							billingResult.AnlasCost = anlasConsumed

							powerConsumed := balBefore.Power - balAfter.Power
							if powerConsumed < 0 {
								powerConsumed = 0
							}
							billingResult.PowerCost = powerConsumed

							if billingResult.PowerCost > 0 && billingResult.AnlasCost > 0 {
								billingResult.Type = service.BillingTypeComposite
							} else if billingResult.PowerCost > 0 {
								billingResult.Type = service.BillingTypePower
							} else if billingResult.AnlasCost > 0 {
								billingResult.Type = service.BillingTypeAnlas
							} else {
								billingResult.Type = service.BillingTypeFree
							}
						}
					}
				}
			}
			if settleErr := service.SettleNovelAIBilling(info.UserId, billingResult); settleErr != nil {
				logger.LogError(c, fmt.Sprintf("SettleNovelAIBilling error: %v", settleErr))
			}
		}
	}

	// 判断下游是 Chat 请求还是 Image 请求
	chatPrompt, isChat := c.Get("novelai_chat_prompt")
	if isChat {
		// 返回 OpenAI Chat Completion 格式
		firstImg := ""
		if len(images) > 0 {
			firstImg = images[0]
		}
		markdownMsg := fmt.Sprintf("![generated image](data:image/png;base64,%s)\n\nPrompt: %s", firstImg, chatPrompt)
		chatResp := dto.OpenAITextResponse{
			Id:      fmt.Sprintf("chatcmpl-%s", common.GetUUID()),
			Object:  "chat.completion",
			Created: time.Now().Unix(),
			Model:   info.OriginModelName,
			Choices: []dto.OpenAITextResponseChoice{
				{
					Index: 0,
					Message: dto.Message{
						Role:    "assistant",
						Content: markdownMsg,
					},
					FinishReason: "stop",
				},
			},
			Usage: dto.Usage{
				PromptTokens:     1,
				CompletionTokens: 1,
				TotalTokens:      2,
			},
		}
		c.JSON(http.StatusOK, chatResp)
		return &chatResp.Usage, nil
	}

	// 否则返回标准 OpenAI Image 格式
	imageResp := dto.ImageResponse{
		Created: time.Now().Unix(),
	}
	for _, b64 := range images {
		imageResp.Data = append(imageResp.Data, dto.ImageData{
			B64Json: b64,
		})
	}

	c.JSON(http.StatusOK, imageResp)
	return &dto.Usage{PromptTokens: 1, TotalTokens: 1}, nil
}

func (a *Adaptor) ConvertClaudeRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.ClaudeRequest) (any, error) {
	return nil, errors.New("claude request not supported on NovelAI")
}

func (a *Adaptor) ConvertGeminiRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeminiChatRequest) (any, error) {
	return nil, errors.New("gemini request not supported on NovelAI")
}

func (a *Adaptor) ConvertRerankRequest(c *gin.Context, relayMode int, request dto.RerankRequest) (any, error) {
	return nil, errors.New("rerank request not supported on NovelAI")
}

func (a *Adaptor) ConvertEmbeddingRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.EmbeddingRequest) (any, error) {
	return nil, errors.New("embedding request not supported on NovelAI")
}

func (a *Adaptor) ConvertAudioRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.AudioRequest) (io.Reader, error) {
	return nil, errors.New("audio request not supported on NovelAI")
}

func (a *Adaptor) ConvertOpenAIResponsesRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.OpenAIResponsesRequest) (any, error) {
	return nil, errors.New("responses request not supported on NovelAI")
}
