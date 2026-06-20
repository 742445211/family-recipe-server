package cache_test
import (
	"recipe-server/internal/cache"
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
)

func setupTestRedis(t *testing.T) (*cache.RedisCache, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)
	c := cache.NewRedisCache(mr.Addr(), "", 0)
	return c, mr
}

func TestSetGetAndTTL(t *testing.T) {
	c, mr := setupTestRedis(t)
	ctx := context.Background()

	if err := c.Set(ctx, "k1", "v1", time.Hour); err != nil {
		t.Fatal(err)
	}
	v, err := c.Get(ctx, "k1")
	if err != nil || v != "v1" {
		t.Fatalf("get: %v %q", err, v)
	}
	if mr.TTL("k1") <= 0 {
		t.Fatal("expected TTL")
	}
}

func TestSetJSONGetJSON(t *testing.T) {
	c, _ := setupTestRedis(t)
	ctx := context.Background()
	type payload struct {
		Name string `json:"name"`
		N    int    `json:"n"`
	}
	in := payload{Name: "番茄炒蛋", N: 2}
	if err := c.SetJSON(ctx, "j1", in, time.Minute); err != nil {
		t.Fatal(err)
	}
	var out payload
	if err := c.GetJSON(ctx, "j1", &out); err != nil {
		t.Fatal(err)
	}
	if out.Name != in.Name || out.N != in.N {
		t.Fatalf("got %+v", out)
	}
}

func TestIncrAndTTL(t *testing.T) {
	c, _ := setupTestRedis(t)
	ctx := context.Background()
	n, err := c.Incr(ctx, "cnt")
	if err != nil || n != 1 {
		t.Fatalf("incr1: %v %d", err, n)
	}
	if err := c.Expire(ctx, "cnt", 2*time.Hour); err != nil {
		t.Fatal(err)
	}
	ttl, err := c.TTL(ctx, "cnt")
	if err != nil || ttl <= 0 {
		t.Fatalf("ttl: %v %v", err, ttl)
	}
	n, err = c.Incr(ctx, "cnt")
	if err != nil || n != 2 {
		t.Fatalf("incr2: %v %d", err, n)
	}
}

func TestDelete(t *testing.T) {
	c, _ := setupTestRedis(t)
	ctx := context.Background()
	_ = c.Set(ctx, "d1", "x", time.Minute)
	if err := c.Delete(ctx, "d1"); err != nil {
		t.Fatal(err)
	}
	_, err := c.Get(ctx, "d1")
	if err != cache.ErrNotFound {
		t.Fatalf("expected cache.ErrNotFound, got %v", err)
	}
}
