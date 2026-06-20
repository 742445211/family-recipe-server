package service_test
import (
	"recipe-server/internal/service"
	"encoding/json"
	"testing"

	"recipe-server/internal/model"
	"recipe-server/internal/testutil"
)

func TestKeyExtensionChanged(t *testing.T) {
	if !service.KeyExtensionChanged("recipe/a.webp", "recipe/a.png") {
		t.Fatal("expected changed")
	}
	if service.KeyExtensionChanged("recipe/a.jpg", "recipe/a.jpg") {
		t.Fatal("expected same")
	}
}

func TestBuildObjectURLNilConfig(t *testing.T) {
	_, err := service.BuildObjectURL("recipe/x.png")
	if err == nil {
		t.Fatal("expected error without config")
	}
}

func TestImageWorkerFridgeRecognizeResult(t *testing.T) {
	db := testutil.SetupTestDB(t)
	userID, familyID := testutil.SeedUserAndFamily(t, db)
	hub := service.NewImageWorkerHub(nil)
	iw := service.NewImageWorkerService(db, hub)
	fridgeSvc := service.NewFridgeService(db, iw)
	iw.SetFridgeRecognizer(fridgeSvc)

	s := model.FridgeScan{
		FamilyID: familyID, UserID: userID, TaskID: "tid-1",
		ImageKey: "k", ImageURL: "u", Status: service.FridgeScanProcessing,
	}
	db.Create(&s)

	payload, _ := json.Marshal(map[string]any{
		"type": "task_result", "task_id": "tid-1", "status": "ok", "action": "recognize",
		"meta": map[string]any{"scope": "fridge", "scan_id": s.ID},
		"detail": map[string]any{
			"items": []map[string]string{{"name": "苹果", "amount": "3个"}},
		},
	})
	iw.HandleTaskResultForTest(payload)

	got, err := fridgeSvc.GetScan(familyID, s.ID)
	if err != nil || got.Status != service.FridgeScanDone {
		t.Fatalf("scan=%+v err=%v", got, err)
	}
	items, _ := service.ScanRecognizedItems(got)
	if len(items) != 1 || items[0].Name != "苹果" {
		t.Fatalf("items=%+v", items)
	}
}

func TestImageWorkerFridgeRecognizeByTaskIDOnly(t *testing.T) {
	db := testutil.SetupTestDB(t)
	userID, familyID := testutil.SeedUserAndFamily(t, db)
	hub := service.NewImageWorkerHub(nil)
	iw := service.NewImageWorkerService(db, hub)
	fridgeSvc := service.NewFridgeService(db, iw)
	iw.SetFridgeRecognizer(fridgeSvc)

	s := model.FridgeScan{
		FamilyID: familyID, UserID: userID, TaskID: "tid-only",
		ImageKey: "k", ImageURL: "u", Status: service.FridgeScanProcessing,
		RecognizedItems: "[]",
	}
	db.Create(&s)

	payload, _ := json.Marshal(map[string]any{
		"type": "result", "task_id": "tid-only", "status": "success",
		"detail": map[string]any{"ingredients": []string{"香蕉", "牛奶"}},
	})
	iw.HandleTaskResultForTest(payload)

	got, err := fridgeSvc.GetScan(familyID, s.ID)
	if err != nil || got.Status != service.FridgeScanDone {
		t.Fatalf("scan=%+v err=%v", got, err)
	}
}

func TestTaskResultOK(t *testing.T) {
	if !service.TaskResultOKForTest("ok") || !service.TaskResultOKForTest("success") || !service.TaskResultOKForTest("SUCCESS") {
		t.Fatal("expected ok statuses")
	}
	if service.TaskResultOKForTest("failed") {
		t.Fatal("expected failed")
	}
}
