package service

import (
	"context"
	"errors"
	"testing"

	"recipe-server/config"
	"recipe-server/internal/cache"
	"recipe-server/internal/model"
	"recipe-server/internal/testutil"

	"github.com/alicebob/miniredis/v2"
)

func setupBlindBoxTest(t *testing.T) (*BlindBoxService, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	store := cache.NewRedisCache(mr.Addr(), "", 0)
	t.Cleanup(mr.Close)
	config.AppConfig = &config.Config{
		BlindBox: config.BlindBoxConfig{
			Enabled: boolPtr(true),
			RateLimit: config.BlindBoxRateLimitConfig{
				Enabled:     true,
				MaxRequests: 5,
				WindowHours: 1,
			},
		},
	}
	return NewBlindBoxService(testutil.SetupTestDB(t), store), mr
}

func boolPtr(v bool) *bool { return &v }

func TestBlindBoxDrawExcludesOrderedRecipes(t *testing.T) {
	svc, _ := setupBlindBoxTest(t)
	db := svc.db
	userID, familyID := testutil.SeedUserAndFamily(t, db)

	r1 := model.Recipe{Name: "A", CreatorID: userID, FamilyID: familyID}
	r2 := model.Recipe{Name: "B", CreatorID: userID, FamilyID: familyID}
	db.Create(&r1)
	db.Create(&r2)

	orderSvc := NewOrderService(db)
	if _, err := orderSvc.Add(familyID, r1.ID, "dinner", userID, "2026-06-12", "", 1); err != nil {
		t.Fatalf("seed order: %v", err)
	}

	res, err := svc.Draw(context.Background(), familyID, userID, "2026-06-12", "dinner", nil)
	if err != nil {
		t.Fatalf("draw: %v", err)
	}
	if res.Recipe.ID != r2.ID {
		t.Fatalf("want recipe B, got %d", res.Recipe.ID)
	}
	if res.PoolSize != 1 {
		t.Fatalf("pool_size want 1, got %d", res.PoolSize)
	}
}

func TestBlindBoxDrawNoCandidates(t *testing.T) {
	svc, _ := setupBlindBoxTest(t)
	db := svc.db
	userID, familyID := testutil.SeedUserAndFamily(t, db)

	r1 := model.Recipe{Name: "Only", CreatorID: userID, FamilyID: familyID}
	db.Create(&r1)
	orderSvc := NewOrderService(db)
	if _, err := orderSvc.Add(familyID, r1.ID, "dinner", userID, "2026-06-12", "", 1); err != nil {
		t.Fatalf("seed order: %v", err)
	}

	_, err := svc.Draw(context.Background(), familyID, userID, "2026-06-12", "dinner", nil)
	if !errors.Is(err, ErrBlindBoxNoCandidates) {
		t.Fatalf("want ErrBlindBoxNoCandidates, got %v", err)
	}
}

func TestBlindBoxDrawExcludeIDs(t *testing.T) {
	svc, _ := setupBlindBoxTest(t)
	db := svc.db
	userID, familyID := testutil.SeedUserAndFamily(t, db)

	r1 := model.Recipe{Name: "A", CreatorID: userID, FamilyID: familyID}
	r2 := model.Recipe{Name: "B", CreatorID: userID, FamilyID: familyID}
	db.Create(&r1)
	db.Create(&r2)

	res, err := svc.Draw(context.Background(), familyID, userID, "2026-06-12", "dinner", []uint64{r1.ID})
	if err != nil {
		t.Fatalf("draw: %v", err)
	}
	if res.Recipe.ID != r2.ID {
		t.Fatalf("exclude should force B, got %d", res.Recipe.ID)
	}
}

func TestBlindBoxRateLimit(t *testing.T) {
	svc, _ := setupBlindBoxTest(t)
	db := svc.db
	userID, familyID := testutil.SeedUserAndFamily(t, db)
	db.Create(&model.Recipe{Name: "X", CreatorID: userID, FamilyID: familyID})

	ctx := context.Background()
	for i := 0; i < 5; i++ {
		if _, err := svc.Draw(ctx, familyID, userID, "2026-06-12", "dinner", nil); err != nil {
			t.Fatalf("draw %d: %v", i, err)
		}
	}
	_, err := svc.Draw(ctx, familyID, userID, "2026-06-12", "dinner", nil)
	if !errors.Is(err, ErrBlindBoxRateLimit) {
		t.Fatalf("want rate limit, got %v", err)
	}
}
