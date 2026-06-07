package notifier

import (
	"context"
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
		RecipeName: "红烧肉",
		AdderName:  "张三",
		MealType:   "dinner",
		Date:       "2026-06-05",
	}
	content := BuildOrderContent(msg)
	if content == "" {
		t.Fatal("content 不应为空")
	}
	if MealName("dinner") != "晚餐" {
		t.Fatal("餐次映射错误")
	}
}
