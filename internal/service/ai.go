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
	Model    string        `json:"model"`
	Messages []ChatMessage `json:"messages"`
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
	if count <= 0 {
		count = 5
	}
	systemPrompt := fmt.Sprintf(`你是家庭私厨推荐助手。请根据以下上下文推荐 %d 道菜。

%s

要求：
1. 优先推荐家庭菜谱中没有的新菜式（至少3道新菜）
2. 保留1-2道用户常点的菜
3. 考虑荤素搭配、营养均衡
4. 结合当前天气推荐合适菜品（热天清爽、冷天暖胃、雨天汤品等）
5. 家常简单、食材易得

必须只返回合法 JSON，不要 markdown 代码块，格式如下：
{"items":[{"name":"菜名","category":"分类","difficulty":"easy或medium或hard","cook_time":分钟数,"ingredients":"[{\"name\":\"食材\",\"amount\":\"用量\"}]","seasonings":"[]","steps":"[\"步骤1\"]","tips":"小贴士","reason":"推荐理由"}]}

注意：ingredients、seasonings、steps 字段必须是 JSON 字符串（与数据库一致），不是嵌套对象。`,
		count, FormatContextBlock(actx))

	model := "deepseek-chat"
	apiKey := ""
	if config.AppConfig != nil {
		if config.AppConfig.AI.Model != "" {
			model = config.AppConfig.AI.Model
		}
		apiKey = config.AppConfig.AI.APIKey
	}

	req := ChatRequest{
		Model: model,
		Messages: []ChatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: "请推荐今晚吃什么，只返回 JSON"},
		},
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
