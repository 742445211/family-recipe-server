package service

import (
	"context"
	"testing"
	"time"

	"recipe-server/config"
	"recipe-server/internal/cache"

	"github.com/alicebob/miniredis/v2"
)

func setupRateLimitTest(t *testing.T, max int, windowH int) (*AIRateLimitService, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(mr.Close)
	config.AppConfig = &config.Config{
		AI: config.AIConfig{
			RateLimit: config.AIRateLimitConfig{
				Enabled:     true,
				MaxRequests: max,
				WindowHours: windowH,
			},
		},
	}
	store := cache.NewRedisCache(mr.Addr(), "", 0)
	return NewAIRateLimitService(store), mr
}

func TestRateLimitAllowsThreeBlocksFourth(t *testing.T) {
	svc, _ := setupRateLimitTest(t, 3, 3)
	ctx := context.Background()
	uid := uint64(42)

	for i := 0; i < 3; i++ {
		st, err := svc.CheckAndConsume(ctx, uid)
		if err != nil {
			t.Fatalf("attempt %d: %v", i+1, err)
		}
		if st.Remaining != 3-int64(i+1) {
			t.Fatalf("remaining: %+v", st)
		}
	}
	_, err := svc.CheckAndConsume(ctx, uid)
	if err != ErrRateLimitExceeded {
		t.Fatalf("expected rate limit, got %v", err)
	}
}

func TestRateLimitDifferentUsers(t *testing.T) {
	svc, _ := setupRateLimitTest(t, 3, 3)
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		if _, err := svc.CheckAndConsume(ctx, 1); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := svc.CheckAndConsume(ctx, 2); err != nil {
		t.Fatalf("user2 should pass: %v", err)
	}
}

func TestRateLimitDisabled(t *testing.T) {
	mr, _ := miniredis.Run()
	t.Cleanup(mr.Close)
	config.AppConfig = &config.Config{
		AI: config.AIConfig{
			RateLimit: config.AIRateLimitConfig{Enabled: false, MaxRequests: 1},
		},
	}
	svc := NewAIRateLimitService(cache.NewRedisCache(mr.Addr(), "", 0))
	for i := 0; i < 5; i++ {
		if _, err := svc.CheckAndConsume(context.Background(), 9); err != nil {
			t.Fatal(err)
		}
	}
}

func TestRateLimitTTLReset(t *testing.T) {
	svc, mr := setupRateLimitTest(t, 3, 3)
	ctx := context.Background()
	uid := uint64(7)
	for i := 0; i < 3; i++ {
		_, _ = svc.CheckAndConsume(ctx, uid)
	}
	mr.FastForward(4 * time.Hour)
	if _, err := svc.CheckAndConsume(ctx, uid); err != nil {
		t.Fatalf("after TTL reset: %v", err)
	}
}
