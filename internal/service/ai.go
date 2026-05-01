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
	systemPrompt := fmt.Sprintf(`你是一个家庭菜谱推荐助手。用户家庭已有菜谱：%v。历史点菜记录：%s。请根据以下原则推荐5道菜：
1. 优先推荐家常、做法简单、食材常见的菜
2. 适当推荐没做过的新菜，丰富餐桌
3. 考虑荤素搭配、营养均衡
4. 返回格式：纯文本，每行一道菜，格式为"菜名 - 推荐理由"`, recipeNames, historySummary)

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
