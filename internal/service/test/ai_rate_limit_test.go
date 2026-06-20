package service_test
import (
	"recipe-server/internal/service"
	"context"
	"testing"
	"time"

	"recipe-server/config"
	"recipe-server/internal/cache"

	"github.com/alicebob/miniredis/v2"
)

func setupRateLimitTest(t *testing.T, scope service.AIRateLimitScope, max int, windowH int) (*service.AIRateLimitService, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(mr.Close)
	rl := config.AIRateLimitConfig{
		Recommend: config.AIRateLimitScopeConfig{Enabled: true, MaxRequests: 5, WindowHours: 2},
		Catalog:   config.AIRateLimitScopeConfig{Enabled: true, MaxRequests: 5, WindowHours: 2},
	}
	switch scope {
	case service.AIRateLimitScopeCatalog:
		rl.Catalog = config.AIRateLimitScopeConfig{Enabled: true, MaxRequests: max, WindowHours: windowH}
	default:
		rl.Recommend = config.AIRateLimitScopeConfig{Enabled: true, MaxRequests: max, WindowHours: windowH}
	}
	config.AppConfig = &config.Config{AI: config.AIConfig{RateLimit: rl}}
	store := cache.NewRedisCache(mr.Addr(), "", 0)
	return service.NewAIRateLimitService(store), mr
}

func TestRateLimitAllowsFiveBlocksSixth(t *testing.T) {
	svc, _ := setupRateLimitTest(t, service.AIRateLimitScopeRecommend, 5, 2)
	ctx := context.Background()
	uid := uint64(42)

	for i := 0; i < 5; i++ {
		st, err := svc.CheckAndConsume(ctx, service.AIRateLimitScopeRecommend, uid)
		if err != nil {
			t.Fatalf("attempt %d: %v", i+1, err)
		}
		if st.Remaining != 5-int64(i+1) {
			t.Fatalf("remaining: %+v", st)
		}
	}
	_, err := svc.CheckAndConsume(ctx, service.AIRateLimitScopeRecommend, uid)
	if err != service.ErrRateLimitExceeded {
		t.Fatalf("expected rate limit, got %v", err)
	}
}

func TestRateLimitRecommendAndCatalogIndependent(t *testing.T) {
	svc, _ := setupRateLimitTest(t, service.AIRateLimitScopeRecommend, 5, 2)
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		if _, err := svc.CheckAndConsume(ctx, service.AIRateLimitScopeRecommend, 1); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := svc.CheckAndConsume(ctx, service.AIRateLimitScopeRecommend, 1); err != service.ErrRateLimitExceeded {
		t.Fatalf("recommend should block: %v", err)
	}
	if _, err := svc.CheckAndConsume(ctx, service.AIRateLimitScopeCatalog, 1); err != nil {
		t.Fatalf("catalog should still pass: %v", err)
	}
}

func TestRateLimitDifferentUsers(t *testing.T) {
	svc, _ := setupRateLimitTest(t, service.AIRateLimitScopeRecommend, 5, 2)
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		if _, err := svc.CheckAndConsume(ctx, service.AIRateLimitScopeRecommend, 1); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := svc.CheckAndConsume(ctx, service.AIRateLimitScopeRecommend, 2); err != nil {
		t.Fatalf("user2 should pass: %v", err)
	}
}

func TestRateLimitDisabled(t *testing.T) {
	mr, _ := miniredis.Run()
	t.Cleanup(mr.Close)
	config.AppConfig = &config.Config{
		AI: config.AIConfig{
			RateLimit: config.AIRateLimitConfig{
				Recommend: config.AIRateLimitScopeConfig{Enabled: false, MaxRequests: 1},
			},
		},
	}
	svc := service.NewAIRateLimitService(cache.NewRedisCache(mr.Addr(), "", 0))
	for i := 0; i < 5; i++ {
		if _, err := svc.CheckAndConsume(context.Background(), service.AIRateLimitScopeRecommend, 9); err != nil {
			t.Fatal(err)
		}
	}
}

func TestRateLimitTTLReset(t *testing.T) {
	svc, mr := setupRateLimitTest(t, service.AIRateLimitScopeRecommend, 5, 2)
	ctx := context.Background()
	uid := uint64(7)
	for i := 0; i < 5; i++ {
		_, _ = svc.CheckAndConsume(ctx, service.AIRateLimitScopeRecommend, uid)
	}
	mr.FastForward(3 * time.Hour)
	if _, err := svc.CheckAndConsume(ctx, service.AIRateLimitScopeRecommend, uid); err != nil {
		t.Fatalf("after TTL reset: %v", err)
	}
}

func TestCatalogRateLimitErrorDistinct(t *testing.T) {
	svc, _ := setupRateLimitTest(t, service.AIRateLimitScopeCatalog, 1, 2)
	ctx := context.Background()
	if _, err := svc.CheckAndConsume(ctx, service.AIRateLimitScopeCatalog, 1); err != nil {
		t.Fatal(err)
	}
	_, err := svc.CheckAndConsume(ctx, service.AIRateLimitScopeCatalog, 1)
	if err != service.ErrCatalogRateLimitExceeded {
		t.Fatalf("got %v", err)
	}
}
