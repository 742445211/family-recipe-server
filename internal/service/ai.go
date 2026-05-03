// Package service - AI 智能推荐服务。
//
// 本文件实现 AI 菜品推荐功能，通过调用兼容 OpenAI API 格式的大语言模型接口，
// 根据家庭已有菜谱和历史点菜记录生成个性化的晚餐推荐。
// 推荐策略：优先推荐新菜式（至少 3 道新菜），兼顾历史偏好（保留 1~2 道常点菜），
// 同时考虑荤素搭配与营养均衡。
package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"recipe-server/config"
)

// AIService AI 推荐服务，封装与大语言模型 API 的交互逻辑。
// 目前为无状态结构体（所有配置来自 config.AppConfig.AI），
// 未来如需添加 HTTP 连接池或缓存可在此扩展。
type AIService struct{}

// NewAIService 创建 AI 服务实例。
//
// 返回值:
//
//	*AIService - AI 服务指针
func NewAIService() *AIService {
	return &AIService{}
}

// ChatMessage 单条对话消息，对应 OpenAI Chat API 的 message 结构。
type ChatMessage struct {
	Role    string `json:"role"`    // 角色：system / user / assistant
	Content string `json:"content"` // 消息正文
}

// ChatRequest OpenAI Chat Completion 请求体。
type ChatRequest struct {
	Model    string        `json:"model"`    // 模型名称（如 gpt-4、deepseek-chat）
	Messages []ChatMessage `json:"messages"` // 对话消息列表
}

// ChatResponse OpenAI Chat Completion 响应体。
type ChatResponse struct {
	Choices []struct {
		Message ChatMessage `json:"message"` // AI 返回的消息
	} `json:"choices"` // 候选回复列表（通常取第一个）
}

// Recommend 根据已有菜谱和历史点菜记录，调用 AI 推荐今晚的 5 道菜。
//
// 参数:
//   - recipeNames   []string - 家庭已有菜谱名称列表
//   - historySummary string   - 历史点菜记录的摘要描述文本
//
// 返回值:
//   - string - AI 推荐的菜品清单（纯文本，每行一道菜，含理由/难度/耗时/主料）
//   - error  - 请求失败、响应解析失败或 AI 未返回结果时返回错误
//
// 说明:
//   - System Prompt 中包含详细推荐约束（新菜优先、荤素搭配等）
//   - 请求目标为 config.AppConfig.AI.BaseURL + "/v1/chat/completions"
//   - 使用 Bearer Token 认证（config.AppConfig.AI.APIKey）
func (s *AIService) Recommend(recipeNames []string, historySummary string) (string, error) {
	// 构建 system prompt：注入家庭菜谱与历史记录作为 AI 上下文
	systemPrompt := fmt.Sprintf(`你是家庭私厨推荐助手。用户家庭已有菜谱：%v。历史点菜记录：%s。

请推荐今晚的 5 道菜，要求：
1. 优先推荐家庭菜谱中没有的新菜式（至少3道新菜），鼓励尝试新口味
2. 如有被频繁点的菜也保留1-2道，兼顾口味偏好
3. 考虑荤素搭配、营养均衡，避免全荤或全素
4. 优先推荐家常、做法简单、食材易得的菜
5. 每道菜说明烹饪难度、大概耗时、主要食材

返回格式（纯文本，每行一道）：
序号. 菜名 - 推荐理由 | 难度：简单/中等/困难 | 耗时：约X分钟 | 主料：食材1、食材2`, recipeNames, historySummary)

	// 构造 Chat Completion 请求
	req := ChatRequest{
		Model: config.AppConfig.AI.Model,
		Messages: []ChatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: "请推荐今晚吃什么"},
		},
	}

	// 序列化请求体为 JSON
	body, _ := json.Marshal(req)

	// 创建 HTTP POST 请求，目标地址为 AI 服务的 chat/completions 端点
	httpReq, _ := http.NewRequest("POST", config.AppConfig.AI.BaseURL+"/v1/chat/completions", bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+config.AppConfig.AI.APIKey)

	// 发送 HTTP 请求
	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("AI请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 解析 AI 返回的 JSON 响应
	var chatResp ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return "", fmt.Errorf("AI响应解析失败: %w", err)
	}

	// 确保 AI 返回了至少一条结果
	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("AI未返回结果")
	}

	// 返回第一条候选回复的文本内容
	return chatResp.Choices[0].Message.Content, nil
}
