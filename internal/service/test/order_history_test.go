package service_test
import (
	"recipe-server/internal/service"
	"testing"
	"time"

	"recipe-server/internal/model"
	"recipe-server/internal/testutil"
)

func TestGetRecentOrderNames(t *testing.T) {
	db := testutil.SetupTestDB(t)
	uid, fid := testutil.SeedUserAndFamily(t, db)
	os := service.NewOrderService(db)

	today := time.Now().Format("2006-01-02")
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")

	r1 := &model.Recipe{Name: "红烧肉", FamilyID: fid, CreatorID: uid, Category: "荤菜",
		Ingredients: `[]`, Steps: `[]`}
	r2 := &model.Recipe{Name: "清炒时蔬", FamilyID: fid, CreatorID: uid, Category: "素菜",
		Ingredients: `[]`, Steps: `[]`}
	db.Create(r1)
	db.Create(r2)

	_, _ = os.Add(fid, r1.ID, "dinner", uid, today, "", 1)
	_, _ = os.Add(fid, r2.ID, "lunch", uid, today, "", 1)
	_, _ = os.Add(fid, r1.ID, "dinner", uid, yesterday, "", 1)

	names, err := os.GetRecentOrderNames(fid, 7, 21)
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 3 {
		t.Fatalf("expected 3 names, got %v", names)
	}
	if names[0] != "红烧肉" || names[1] != "清炒时蔬" {
		t.Fatalf("order: %v", names)
	}
}

func TestGetRecentOrderNamesRespectsLimit(t *testing.T) {
	db := testutil.SetupTestDB(t)
	uid, fid := testutil.SeedUserAndFamily(t, db)
	os := service.NewOrderService(db)
	today := time.Now().Format("2006-01-02")

	for i := 0; i < 5; i++ {
		r := &model.Recipe{Name: "菜" + string(rune('A'+i)), FamilyID: fid, CreatorID: uid,
			Ingredients: `[]`, Steps: `[]`}
		db.Create(r)
		_, _ = os.Add(fid, r.ID, "dinner", uid, today, "", 1)
	}
	names, err := os.GetRecentOrderNames(fid, 7, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 3 {
		t.Fatalf("got %d", len(names))
	}
}
