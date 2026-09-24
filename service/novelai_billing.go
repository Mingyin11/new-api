package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/types"
)

type NovelAIBillingType int

const (
	BillingTypeFree          NovelAIBillingType = iota // 免费图 (扣除 0)
	BillingTypePower                                   // 电量生图 (消耗 Power)
	BillingTypeAnlas                                   // Anlas付费图 (消耗 Anlas)
	BillingTypeComposite                               // 复合扣费 (同时消耗 Power 和 Anlas)
)

func (b NovelAIBillingType) String() string {
	switch b {
	case BillingTypeFree:
		return "free"
	case BillingTypePower:
		return "power"
	case BillingTypeAnlas:
		return "anlas"
	case BillingTypeComposite:
		return "composite"
	default:
		return "unknown"
	}
}

type NovelAIParamSpec struct {
	Model  string
	Width  int
	Height int
	Steps  int
	N      int
	Action string
}

type NovelAIBillingResult struct {
	Model     string             `json:"model"`
	Type      NovelAIBillingType `json:"type"`
	PowerCost int                `json:"power_cost"`
	AnlasCost int                `json:"anlas_cost"`
}

func (r NovelAIBillingResult) String() string {
	switch r.Type {
	case BillingTypeFree:
		return fmt.Sprintf("free(%s: 0)", r.Model)
	case BillingTypePower:
		return fmt.Sprintf("power(%s: %d Power)", r.Model, r.PowerCost)
	case BillingTypeAnlas:
		return fmt.Sprintf("anlas(%s: %d Anlas)", r.Model, r.AnlasCost)
	case BillingTypeComposite:
		return fmt.Sprintf("composite(%s: %d Power + %d Anlas)", r.Model, r.PowerCost, r.AnlasCost)
	default:
		return "unknown"
	}
}

func (r NovelAIBillingResult) LogString() string {
	switch r.Type {
	case BillingTypeFree:
		return fmt.Sprintf("【NovelAI】%s 免费规格 (0 扣除)", r.Model)
	case BillingTypePower:
		return fmt.Sprintf("【NovelAI】%s 扣除 %d 每日电量 (Power)", r.Model, r.PowerCost)
	case BillingTypeAnlas:
		return fmt.Sprintf("【NovelAI】%s 扣除 %d Opus 点数 (Anlas)", r.Model, r.AnlasCost)
	case BillingTypeComposite:
		return fmt.Sprintf("【NovelAI】%s 扣除 %d 每日电量 (Power) + %d Opus 点数 (Anlas)", r.Model, r.PowerCost, r.AnlasCost)
	default:
		return fmt.Sprintf("【NovelAI】%s 已完成生成", r.Model)
	}
}

// ClassifyNovelAIRequest 精准判定请求属于 免费图、电量图、Anlas图 还是 复合图
// 定价规则：
// 1. nai-diffusion-3: 免费图 (0 扣除)
// 2. nai-diffusion-4-full: 5 Anlas 图 (5 Anlas * N)
// 3. nai-diffusion-4-5-full: 1 电量图 (1 Power * N)
// 4. nai-diffusion-5: 1 电量 + 5 Anlas 图 (1 Power * N + 5 Anlas * N)
func ClassifyNovelAIRequest(spec NovelAIParamSpec) NovelAIBillingResult {
	modelName := strings.ToLower(strings.TrimSpace(spec.Model))
	n := spec.N
	if n <= 0 {
		n = 1
	}

	result := NovelAIBillingResult{
		Model: spec.Model,
	}

	// 1. nai-diffusion-3 代表免费图
	if strings.Contains(modelName, "nai-diffusion-3") || modelName == "nai-diffusion-3" {
		result.Type = BillingTypeFree
		result.PowerCost = 0
		result.AnlasCost = 0
		return result
	}

	// 2. nai-diffusion-4-5-full 代表 1 电量图 (注意：必须在 4-full 之前优先匹配 4-5)
	if strings.Contains(modelName, "nai-diffusion-4-5") || strings.Contains(modelName, "nai-diffusion-4.5") {
		result.Type = BillingTypePower
		result.PowerCost = 1 * n
		result.AnlasCost = 0
		return result
	}

	// 3. nai-diffusion-4-full 代表 5 Anlas 图
	if strings.Contains(modelName, "nai-diffusion-4") || strings.Contains(modelName, "nai-diffusion-4-full") {
		result.Type = BillingTypeAnlas
		result.PowerCost = 0
		result.AnlasCost = 5 * n
		return result
	}

	// 4. 5 模型 (nai-diffusion-5 / V5)：每调用一次电量 -1 (消耗 1 Power * n, 0 Anlas)
	if strings.Contains(modelName, "nai-diffusion-5") || strings.Contains(modelName, "diffusion-5") || modelName == "v5" {
		result.Type = BillingTypePower
		result.PowerCost = 1 * n
		result.AnlasCost = 0
		return result
	}

	// 兜底通用规则判定
	width := spec.Width
	height := spec.Height
	steps := spec.Steps
	if width <= 0 {
		width = 832
	}
	if height <= 0 {
		height = 1216
	}
	if steps <= 0 {
		steps = 28
	}
	pixels := width * height

	// Opus 免费图规格
	if pixels <= 1048576 && steps <= 28 && n == 1 && (spec.Action == "" || spec.Action == "generate") {
		result.Type = BillingTypeFree
		result.PowerCost = 0
		result.AnlasCost = 0
		return result
	}

	// 超规扣费 (按标准公式算 Anlas)
	costPerImg := int(math.Ceil(float64(pixels*steps) * 0.00000057))
	if costPerImg < 2 {
		costPerImg = 2
	}
	result.Type = BillingTypeAnlas
	result.PowerCost = 0
	result.AnlasCost = costPerImg * n
	return result
}

// PreCheckNovelAIBilling 前置检查用户余额是否支持当前生成
func PreCheckNovelAIBilling(userId int, result NovelAIBillingResult) *types.NewAPIError {
	if result.Type == BillingTypeFree || (result.PowerCost <= 0 && result.AnlasCost <= 0) {
		// 免费图即使余额为负也直接放行
		return nil
	}

	user, err := model.GetUserById(userId, false)
	if err != nil {
		return types.NewError(err, types.ErrorCodeQueryDataError)
	}

	// 触发惰性刷新（如果今日尚未刷新电量或本月尚未分配 Anlas）
	_, _ = model.CheckAndRefreshUserNovelAI(user, "", "")

	// 1. 电量检查: 若需要消耗电量，电量余额必须 >= 0
	if result.PowerCost > 0 {
		if user.Power < 0 {
			return types.NewErrorWithStatusCode(
				fmt.Errorf("当前电量余额为负数 (%d)，无法生成图片，请等待每日电量刷新或联系管理员", user.Power),
				types.ErrorCodeInsufficientUserQuota,
				http.StatusPaymentRequired,
				types.ErrOptionWithSkipRetry(),
			)
		}
	}

	// 2. Anlas 检查: 若需要消耗 Anlas，Anlas 余额必须 >= 0
	if result.AnlasCost > 0 {
		if user.Anlas < 0 {
			return types.NewErrorWithStatusCode(
				fmt.Errorf("当前 Anlas 余额为负数 (%d)，无法进行付费生图，请充值后重试", user.Anlas),
				types.ErrorCodeInsufficientUserQuota,
				http.StatusPaymentRequired,
				types.ErrOptionWithSkipRetry(),
			)
		}
	}

	return nil
}

// SettleNovelAIBilling 后置实际扣减 (扣减后允许为负数)
func SettleNovelAIBilling(userId int, result NovelAIBillingResult) error {
	if result.PowerCost > 0 {
		if err := model.AdjustUserPower(userId, -int64(result.PowerCost)); err != nil {
			return err
		}
	}
	if result.AnlasCost > 0 {
		if err := model.AdjustUserAnlas(userId, -int64(result.AnlasCost)); err != nil {
			return err
		}
	}
	return nil
}

type NovelAISubscriptionResponse struct {
	Tier              int  `json:"tier"`
	Active            bool `json:"active"`
	TrainingStepsLeft struct {
		FixedTrainingStepsLeft int `json:"fixedTrainingStepsLeft"`
		PurchasedTrainingSteps int `json:"purchasedTrainingSteps"`
	} `json:"trainingStepsLeft"`
}

// FetchOfficialAnlasBalance 调用官方 GET /user/subscription 接口获取官方账号实际剩余的 Anlas 总点数
// (fixedTrainingStepsLeft + purchasedTrainingSteps)
func FetchOfficialAnlasBalance(ctx context.Context, baseURL string, apiKey string) (int, error) {
	base := strings.TrimRight(baseURL, "/")
	if base == "" {
		base = "https://image.novelai.net"
	}
	url := fmt.Sprintf("%s/user/subscription", base)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, err
	}
	cleanToken := strings.TrimSpace(apiKey)
	if !strings.HasPrefix(cleanToken, "Bearer ") {
		cleanToken = "Bearer " + cleanToken
	}
	req.Header.Set("Authorization", cleanToken)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("GET %s status %d: %s", url, resp.StatusCode, string(body))
	}

	var sub NovelAISubscriptionResponse
	if err := json.NewDecoder(resp.Body).Decode(&sub); err != nil {
		return 0, err
	}
	totalAnlas := sub.TrainingStepsLeft.FixedTrainingStepsLeft + sub.TrainingStepsLeft.PurchasedTrainingSteps
	return totalAnlas, nil
}

