package service

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestWebSocketHubOnConnect(t *testing.T) {
	hub := NewWebSocketHub()
	var called atomic.Uint64
	hub.SetOnConnect(func(userID uint64) {
		called.Store(userID)
	})
	hub.triggerOnConnect(123)
	deadline := time.Now().Add(500 * time.Millisecond)
	for called.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if called.Load() != 123 {
		t.Fatalf("onConnect userID: want 123, got %d", called.Load())
	}
}

func TestWebSocketHubPushOfflineSkipped(t *testing.T) {
	hub := NewWebSocketHub()
	if hub.IsOnline(999) {
		t.Fatal("未连接用户不应在线")
	}
	if hub.PushToUser(999, map[string]any{"type": "ORDER_CREATED"}) {
		t.Fatal("离线用户推送应失败")
	}
}
