// Package service - AI 智能推荐服务。
package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"recipe-server/config"
)

// AIService AI 推荐服务，封装与大语言模型 API 的交互逻辑。
type AIService struct {
	client HTTPDoer
	baseURL string
}

// NewAIService 创建 AI 服务实例。
func NewAIService() *AIService {
	return NewAIServiceWithClient(&http.Client{Timeout: 120 * time.Second})
}

// NewAIServiceWithClient 可注入 HTTP 客户端（测试用）。
func NewAIServiceWithClient(client HTTPDoer) *AIService {
	base := "https://api.deepseek.com"
	if config.AppConfig != nil && config.AppConfig.AI.BaseURL != "" {
		base = strings.TrimSuffix(config.AppConfig.AI.BaseURL, "/")
	}
	return &AIService{client: client, baseURL: base}
}

// ChatMessage 单条对话消息。
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatRequest OpenAI Chat Completion 请求体。
type ChatRequest struct {
	Model          string        `json:"model"`
	Messages       []ChatMessage `json:"messages"`
	ResponseFormat *struct {
		Type string `json:"type"`
	} `json:"response_format,omitempty"`
}

// ChatResponse OpenAI Chat Completion 响应体。
type ChatResponse struct {
	Choices []struct {
		Message ChatMessage `json:"message"`
	} `json:"choices"`
}

// Recommend 兼容旧接口：纯文本推荐。
func (s *AIService) Recommend(recipeNames []string, historySummary string) (string, error) {
	ctx := &AIRecommendContext{
		RecipeNames:  recipeNames,
		OrderHistory: strings.Split(strings.TrimPrefix(historySummary, "最近点过的菜："), "、"),
		WeatherLine:  "天气信息暂不可用",
	}
	if historySummary == "暂无历史记录" {
		ctx.OrderHistory = nil
	}
	return s.RecommendStructured(ctx, 5)
}

// RecommendStructured 结构化 JSON 推荐。
func (s *AIService) RecommendStructured(actx *AIRecommendContext, count int) (string, error) {
	return s.recommendStructured(actx, count, "")
}

func (s *AIService) recommendStructured(actx *AIRecommendContext, count int, userHint string) (string, error) {
	if count <= 0 {
		count = 5
	}
	newMin := count
	if newMin > 1 {
		newMin = count - 1 // 至少 N-1 道必须是新菜
	}
	mealName := "本餐"
	if actx != nil && actx.Meal.Name != "" {
		mealName = actx.Meal.Name
	}
	systemPrompt := fmt.Sprintf(`你是家庭私厨推荐助手，请结合下面的上下文，为「%s」推荐 %d 道菜。

%s

推荐要求：
1. 【餐次匹配】所有菜品都要贴合当前餐次「%s」的特点：
   - 早餐：清淡快手、好消化（粥/面点/蛋奶/豆浆/小菜等）
   - 午餐/晚餐：有主菜也有配菜，荤素搭配、营养均衡
   - 宵夜：分量适中、不过于油腻（小吃/汤面/卤味/小烧烤等）
2. 【全新菜式】每道 name 都必须是「家庭已有菜谱」列表里完全没有的新菜名，禁止同名或仅差一两个字的变体（如已有「番茄炒蛋」则不可推荐「西红柿炒蛋」）；至少 %d 道为全新菜式，也不要推荐用户最近点过的菜
3. 【丰富多样】菜品之间在菜系、口味、烹饪方式（炒/炖/蒸/凉拌/汤/烤等）和主食材上尽量不重复，避免连续多道同类
4. 【因地制宜】结合当前天气与季节灵活调整（热天清爽、冷天暖胃、雨天汤品、应季食材等）
5. 家常易做、食材易得；reason 字段要结合「餐次/天气/为什么是新菜」给出贴合今天的推荐理由

必须只返回合法 JSON，不要 markdown 代码块，格式如下：
{"items":[{"name":"菜名","category":"分类","meal_type":"breakfast或lunch或dinner或supper","difficulty":"easy或medium或hard","cook_time":分钟数,"ingredients":"[{\"name\":\"食材\",\"amount\":\"用量\"}]","seasonings":"[]","steps":"[\"步骤1\"]","tips":"小贴士","reason":"推荐理由"}]}

注意：ingredients、seasonings、steps 字段必须是 JSON 字符串（与数据库一致），不是嵌套对象；meal_type 填该菜最适合的餐次。`,
		mealName, count, FormatContextBlock(actx), mealName, newMin)

	model := "deepseek-chat"
	apiKey := ""
	if config.AppConfig != nil {
		if config.AppConfig.AI.Model != "" {
			model = config.AppConfig.AI.Model
		}
		apiKey = config.AppConfig.AI.APIKey
	}

	userMsg := fmt.Sprintf("请为「%s」推荐家庭中从未出现的新菜，只返回 JSON", mealName)
	if strings.TrimSpace(userHint) != "" {
		userMsg = strings.TrimSpace(userHint) + "，只返回 JSON"
	}
	req := ChatRequest{
		Model: model,
		Messages: []ChatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userMsg},
		},
		ResponseFormat: &struct {
			Type string `json:"type"`
		}{Type: "json_object"},
	}
	body, _ := json.Marshal(req)
	httpReq, err := http.NewRequest(http.MethodPost, s.baseURL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := s.client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("AI请求失败: %w", err)
	}
	defer resp.Body.Close()

	var chatResp ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return "", fmt.Errorf("AI响应解析失败: %w", err)
	}
	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("AI未返回结果")
	}
	return chatResp.Choices[0].Message.Content, nil
}
