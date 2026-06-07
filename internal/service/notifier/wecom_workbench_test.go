package notifier

import (
	"context"
	"strings"
	"testing"
)

func TestWecomWorkbenchMissingUserid(t *testing.T) {
	n := NewWecomWorkbenchNotifier(true, nil)
	_, err := n.Send(context.Background(), NotificationMessage{Title: "t", Content: "c"}, NotificationTarget{})
	if err == nil {
		t.Fatal("缺少 userid 应返回错误")
	}
}

func TestBuildOrderContent(t *testing.T) {
	msg := NotificationMessage{
		RecipeName:  "红烧肉",
		AdderName:   "张三",
		MealType:    "dinner",
		Date:        "2026-06-05",
		Ingredients: `[{"name":"番茄","amount":"2个"},{"name":"鸡蛋","amount":"3个"}]`,
		Note:        "少油",
	}
	content := BuildOrderContent(msg)
	if content == "" {
		t.Fatal("content 不应为空")
	}
	if MealName("dinner") != "晚餐" {
		t.Fatal("餐次映射错误")
	}
	for _, want := range []string{"2026-06-05", "晚餐", "红烧肉", "张三", "食材：", "番茄2个", "鸡蛋3个", "备注：少油"} {
		if !strings.Contains(content, want) {
			t.Fatalf("content 应包含 %q，实际: %s", want, content)
		}
	}
}

func TestFormatIngredients(t *testing.T) {
	got := FormatIngredients(`[{"name":"番茄","amount":"2个"},{"name":"鸡蛋","amount":"3个"}]`)
	if got != "番茄2个、鸡蛋3个" {
		t.Fatalf("FormatIngredients: got %q", got)
	}
	if FormatIngredients("") != "" {
		t.Fatal("空食材应返回空字符串")
	}
}
