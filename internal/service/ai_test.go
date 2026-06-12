package service

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"recipe-server/config"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestAIServiceRecommendStructured(t *testing.T) {
	oldCfg := config.AppConfig
	config.AppConfig = &config.Config{
		AI: config.AIConfig{Model: "test-model", APIKey: "key"},
	}
	t.Cleanup(func() { config.AppConfig = oldCfg })

	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if !strings.HasSuffix(r.URL.Path, "/v1/chat/completions") {
			t.Fatalf("path: %s", r.URL.Path)
		}
		body := `{"choices":[{"message":{"content":"{\"items\":[]}"}}]}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})}

	svc := NewAIServiceWithClient(client)
	svc.baseURL = "http://ai.test"
	out, err := svc.RecommendStructured(&AIRecommendContext{
		RecipeNames: []string{"番茄炒蛋"},
		WeatherLine: "晴",
	}, 3)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "items") {
		t.Fatalf("unexpected: %q", out)
	}
}

func TestAIServiceRecommendStructuredEmptyChoices(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"choices":[]}`)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})}
	svc := NewAIServiceWithClient(client)
	svc.baseURL = "http://ai.test"
	_, err := svc.RecommendStructured(&AIRecommendContext{}, 1)
	if err == nil {
		t.Fatal("空 choices 应返回错误")
	}
}

func TestAIServiceGenerateRecipeByName(t *testing.T) {
	oldCfg := config.AppConfig
	config.AppConfig = &config.Config{
		AI: config.AIConfig{Model: "test-model", APIKey: "key"},
	}
	t.Cleanup(func() { config.AppConfig = oldCfg })

	body := `{"items":[{"name":"番茄炒蛋","category":"家常菜","difficulty":"easy","cook_time":15,"ingredients":"[]","seasonings":"[]","steps":"[\"炒\"]","tips":"","reason":""}]}`
	quoted, _ := json.Marshal(body)
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"choices":[{"message":{"content":` + string(quoted) + `}}]}`)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})}

	svc := NewAIServiceWithClient(client)
	svc.baseURL = "http://ai.test"
	out, err := svc.GenerateRecipeByName("番茄炒蛋", nil)
	if err != nil {
		t.Fatal(err)
	}
	if out.Name != "番茄炒蛋" || out.Category != "家常菜" {
		t.Fatalf("%+v", out)
	}
}

func TestAIServiceRecommendLegacy(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		body := `{"choices":[{"message":{"content":"{\"items\":[{\"name\":\"新菜\"}]}"}}]}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})}
	svc := NewAIServiceWithClient(client)
	svc.baseURL = "http://ai.test"
	out, err := svc.Recommend(nil, "暂无历史记录")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "新菜") {
		t.Fatalf("unexpected: %q", out)
	}
}
