package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"recipe-server/config"
)

type AIService struct{}

func NewAIService() *AIService {
	return &AIService{}
}

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatRequest struct {
	Model    string        `json:"model"`
	Messages []ChatMessage `json:"messages"`
}

type ChatResponse struct {
	Choices []struct {
		Message ChatMessage `json:"message"`
	} `json:"choices"`
}

// Recommend 根据已有菜谱和历史点菜推荐
func (s *AIService) Recommend(recipeNames []string, historySummary string) (string, error) {
	systemPrompt := fmt.Sprintf(`你是家庭私厨推荐助手。用户家庭已有菜谱：%v。历史点菜记录：%s。

请推荐今晚的 5 道菜，要求：
1. 优先推荐家庭菜谱中没有的新菜式（至少3道新菜），鼓励尝试新口味
2. 如有被频繁点的菜也保留1-2道，兼顾口味偏好
3. 考虑荤素搭配、营养均衡，避免全荤或全素
4. 优先推荐家常、做法简单、食材易得的菜
5. 每道菜说明烹饪难度、大概耗时、主要食材

返回格式（纯文本，每行一道）：
序号. 菜名 - 推荐理由 | 难度：简单/中等/困难 | 耗时：约X分钟 | 主料：食材1、食材2`, recipeNames, historySummary)

	req := ChatRequest{
		Model: config.AppConfig.AI.Model,
		Messages: []ChatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: "请推荐今晚吃什么"},
		},
	}

	body, _ := json.Marshal(req)
	httpReq, _ := http.NewRequest("POST", config.AppConfig.AI.BaseURL+"/v1/chat/completions", bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+config.AppConfig.AI.APIKey)

	resp, err := http.DefaultClient.Do(httpReq)
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
