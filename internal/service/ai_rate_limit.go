package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"recipe-server/config"
	"recipe-server/internal/cache"
)

var ErrRateLimitExceeded = errors.New("AI推荐次数已达上限")

// RateLimitStatus 限流状态。
type RateLimitStatus struct {
	Limit            int   `json:"limit"`
	Used             int64 `json:"used"`
	Remaining        int64 `json:"remaining"`
	ResetAfterSec    int64 `json:"reset_after_sec"`
	RetryAfterSec    int64 `json:"retry_after_sec,omitempty"`
}

// AIRateLimitService AI 推荐限流（按 user_id）。
type AIRateLimitService struct {
	store cache.Store
}

func NewAIRateLimitService(store cache.Store) *AIRateLimitService {
	return &AIRateLimitService{store: store}
}

func rateLimitKey(userID uint64) string {
	return fmt.Sprintf("ai:ratelimit:user:%d", userID)
}

func (s *AIRateLimitService) cfg() config.AIRateLimitConfig {
	if config.AppConfig == nil {
		return config.AIRateLimitConfig{Enabled: true, MaxRequests: 3, WindowHours: 3}
	}
	return config.AppConfig.AI.RateLimit
}

// CheckAndConsume 检查配额并消耗一次（在调用 LLM 前执行）。
func (s *AIRateLimitService) CheckAndConsume(ctx context.Context, userID uint64) (*RateLimitStatus, error) {
	c := s.cfg()
	if !c.Enabled {
		return &RateLimitStatus{Limit: c.MaxRequests, Used: 0, Remaining: int64(c.MaxRequests)}, nil
	}
	key := rateLimitKey(userID)
	window := time.Duration(c.WindowHours) * time.Hour

	raw, err := s.store.Get(ctx, key)
	var used int64
	if err == nil {
		fmt.Sscanf(raw, "%d", &used)
	}
	if used >= int64(c.MaxRequests) {
		ttl, _ := s.store.TTL(ctx, key)
		sec := int64(ttl.Seconds())
		if sec < 0 {
			sec = 0
		}
		return &RateLimitStatus{
			Limit:         c.MaxRequests,
			Used:          used,
			Remaining:     0,
			ResetAfterSec: sec,
			RetryAfterSec: sec,
		}, ErrRateLimitExceeded
	}

	n, err := s.store.Incr(ctx, key)
	if err != nil {
		return nil, err
	}
	if n == 1 {
		_ = s.store.Expire(ctx, key, window)
	}
	ttl, _ := s.store.TTL(ctx, key)
	sec := int64(ttl.Seconds())
	if sec < 0 {
		sec = int64(window.Seconds())
	}
	return &RateLimitStatus{
		Limit:         c.MaxRequests,
		Used:          n,
		Remaining:     int64(c.MaxRequests) - n,
		ResetAfterSec: sec,
	}, nil
}

// Peek 只读当前限流状态（不消耗配额）。
func (s *AIRateLimitService) Peek(ctx context.Context, userID uint64) (*RateLimitStatus, error) {
	c := s.cfg()
	key := rateLimitKey(userID)
	raw, err := s.store.Get(ctx, key)
	var used int64
	if err == cache.ErrNotFound {
		used = 0
	} else if err != nil {
		return nil, err
	} else {
		fmt.Sscanf(raw, "%d", &used)
	}
	ttl, _ := s.store.TTL(ctx, key)
	sec := int64(ttl.Seconds())
	if sec < 0 {
		sec = int64(time.Duration(c.WindowHours) * time.Hour / time.Second)
	}
	rem := int64(c.MaxRequests) - used
	if rem < 0 {
		rem = 0
	}
	return &RateLimitStatus{
		Limit:         c.MaxRequests,
		Used:          used,
		Remaining:     rem,
		ResetAfterSec: sec,
	}, nil
}
