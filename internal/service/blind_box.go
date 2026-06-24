// Package service - 点菜盲盒。
//
// 从家庭可用菜谱池中随机抽取一道（可排除已点/指定 ID），Redis 限流控制抽取频率。
package service

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"time"

	"recipe-server/config"
	"recipe-server/internal/cache"
	"recipe-server/internal/model"

	"gorm.io/gorm"
)

var (
	ErrBlindBoxNoCandidates = errors.New("没有可选的菜了")
	ErrBlindBoxDisabled     = errors.New("盲盒功能未开启")
	ErrBlindBoxRateLimit    = errors.New("开盲盒次数已达上限")
)

// BlindBoxDrawResult 盲盒抽取结果。
type BlindBoxDrawResult struct {
	Recipe    model.Recipe    `json:"recipe"`
	PoolSize  int             `json:"pool_size"`
	RateLimit *RateLimitStatus `json:"rate_limit"`
}

// BlindBoxService 点菜盲盒：从家庭可用菜谱中随机抽取。
type BlindBoxService struct {
	db    *gorm.DB
	store cache.Store
}

func NewBlindBoxService(db *gorm.DB, store cache.Store) *BlindBoxService {
	return &BlindBoxService{db: db, store: store}
}

func (s *BlindBoxService) cfg() config.BlindBoxRateLimitConfig {
	def := config.BlindBoxRateLimitConfig{Enabled: true, MaxRequests: 30, WindowHours: 3}
	if config.AppConfig == nil {
		return def
	}
	c := config.AppConfig.BlindBox.RateLimit
	if c.MaxRequests == 0 {
		c.MaxRequests = def.MaxRequests
	}
	if c.WindowHours == 0 {
		c.WindowHours = def.WindowHours
	}
	return c
}

func blindBoxRateKey(userID uint64) string {
	return fmt.Sprintf("blindbox:ratelimit:user:%d", userID)
}

func (s *BlindBoxService) consumeRate(ctx context.Context, userID uint64) (*RateLimitStatus, error) {
	c := s.cfg()
	if !c.Enabled || s.store == nil {
		return &RateLimitStatus{Limit: c.MaxRequests, Remaining: int64(c.MaxRequests)}, nil
	}
	key := blindBoxRateKey(userID)
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
		}, ErrBlindBoxRateLimit
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

// Draw 随机抽取一道尚未点过的菜。
func (s *BlindBoxService) Draw(ctx context.Context, familyID, userID uint64, date, mealType string, excludeIDs []uint64) (*BlindBoxDrawResult, error) {
	if config.AppConfig != nil && !config.AppConfig.BlindBoxEnabled() {
		return nil, ErrBlindBoxDisabled
	}
	if mealType == "" {
		mealType = "dinner"
	}

	rl, err := s.consumeRate(ctx, userID)
	if err != nil {
		return &BlindBoxDrawResult{RateLimit: rl}, err
	}

	candidates, err := s.listCandidates(familyID, date, mealType, excludeIDs)
	if err != nil {
		return nil, err
	}
	if len(candidates) == 0 {
		return &BlindBoxDrawResult{RateLimit: rl}, ErrBlindBoxNoCandidates
	}

	idx, err := rand.Int(rand.Reader, big.NewInt(int64(len(candidates))))
	if err != nil {
		return nil, err
	}
	picked := candidates[idx.Int64()]

	return &BlindBoxDrawResult{
		Recipe:    picked,
		PoolSize:  len(candidates),
		RateLimit: rl,
	}, nil
}

func (s *BlindBoxService) listCandidates(familyID uint64, date, mealType string, excludeIDs []uint64) ([]model.Recipe, error) {
	var orderedIDs []uint64
	if err := s.db.Model(&model.DailyOrder{}).
		Where("family_id = ? AND date = ? AND meal_type = ?", familyID, date, mealType).
		Pluck("recipe_id", &orderedIDs).Error; err != nil {
		return nil, err
	}

	skip := make(map[uint64]struct{}, len(orderedIDs)+len(excludeIDs))
	for _, id := range orderedIDs {
		skip[id] = struct{}{}
	}
	for _, id := range excludeIDs {
		skip[id] = struct{}{}
	}

	var recipes []model.Recipe
	q := s.db.Where("(family_id = ? OR is_public = ?)", familyID, true)
	if len(skip) > 0 {
		ids := make([]uint64, 0, len(skip))
		for id := range skip {
			ids = append(ids, id)
		}
		q = q.Where("id NOT IN ?", ids)
	}
	if err := q.Order("id ASC").Find(&recipes).Error; err != nil {
		return nil, err
	}
	return recipes, nil
}
