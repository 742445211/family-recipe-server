package service_test
import (
	"recipe-server/internal/service"
	"encoding/json"
	"errors"
	"testing"

	"recipe-server/internal/model"
	"recipe-server/internal/testutil"
)

type mockFridgeDispatcher struct {
	dispatched bool
	connected  bool
	lastScanID uint64
	lastTaskID string
}

func (m *mockFridgeDispatcher) DispatchFridgeRecognize(scanID uint64, taskID, ossKey, ossURL string) bool {
	m.lastScanID = scanID
	m.lastTaskID = taskID
	return m.dispatched
}

func (m *mockFridgeDispatcher) IsWorkerConnected() bool {
	if m.connected {
		return true
	}
	return m.dispatched
}

func TestFridgeItemCRUD(t *testing.T) {
	db := testutil.SetupTestDB(t)
	userID, familyID := testutil.SeedUserAndFamily(t, db)
	svc := service.NewFridgeService(db, &mockFridgeDispatcher{dispatched: true})

	item, err := svc.CreateItem(familyID, userID, service.FridgeItemInput{
		Name: "鸡蛋", Amount: "6个", Note: "冷藏",
	})
	if err != nil || item.ID == 0 || item.Source != service.FridgeSourceManual {
		t.Fatalf("create: %+v err=%v", item, err)
	}

	list, err := svc.ListItems(familyID)
	if err != nil || len(list) != 1 {
		t.Fatalf("list=%+v err=%v", list, err)
	}

	updated, err := svc.UpdateItem(familyID, item.ID, service.FridgeItemInput{Name: "鸡蛋", Amount: "4个"})
	if err != nil || updated.Amount != "4个" {
		t.Fatalf("update: %+v err=%v", updated, err)
	}

	if err := svc.DeleteItem(familyID, item.ID); err != nil {
		t.Fatal(err)
	}
	list, _ = svc.ListItems(familyID)
	if len(list) != 0 {
		t.Fatalf("expected empty after delete, got %d", len(list))
	}
}

func TestFridgeCreateScanDispatch(t *testing.T) {
	db := testutil.SetupTestDB(t)
	userID, familyID := testutil.SeedUserAndFamily(t, db)
	disp := &mockFridgeDispatcher{dispatched: true}
	svc := service.NewFridgeService(db, disp)

	scan, err := svc.CreateScan(userID, familyID, "fridge/a.jpg", "https://cdn/a.jpg")
	if err != nil {
		t.Fatal(err)
	}
	if scan.Status != service.FridgeScanProcessing || scan.TaskID == "" {
		t.Fatalf("scan=%+v", scan)
	}
	if scan.RecognizedItems != "[]" {
		t.Fatalf("recognized_items should be [] on create, got %q", scan.RecognizedItems)
	}
	if disp.lastScanID != scan.ID || disp.lastTaskID != scan.TaskID {
		t.Fatalf("dispatch scan_id=%d task=%s", disp.lastScanID, disp.lastTaskID)
	}
}

func TestFridgeCreateScanWorkerOffline(t *testing.T) {
	db := testutil.SetupTestDB(t)
	userID, familyID := testutil.SeedUserAndFamily(t, db)
	svc := service.NewFridgeService(db, &mockFridgeDispatcher{dispatched: false})

	scan, err := svc.CreateScan(userID, familyID, "fridge/b.jpg", "https://cdn/b.jpg")
	if !errors.Is(err, service.ErrFridgeWorkerOffline) {
		t.Fatalf("want offline err, got %v scan=%+v", err, scan)
	}
	if scan.Status != service.FridgeScanFailed {
		t.Fatalf("status=%s", scan.Status)
	}
}

func TestParseRecognizeDetailItemsAndLegacy(t *testing.T) {
	items, err := service.ParseRecognizeDetail(json.RawMessage(`{"items":[{"name":"鸡蛋","amount":"6个"}]}`))
	if err != nil || len(items) != 1 || items[0].Name != "鸡蛋" {
		t.Fatalf("items=%+v err=%v", items, err)
	}

	items, err = service.ParseRecognizeDetail(json.RawMessage(`{"ingredients":["番茄","牛奶"]}`))
	if err != nil || len(items) != 2 || items[0].Amount != "" {
		t.Fatalf("legacy=%+v err=%v", items, err)
	}

	items, err = service.ParseRecognizeDetail(json.RawMessage(`["苹果","梨"]`))
	if err != nil || len(items) != 2 || items[0].Name != "苹果" {
		t.Fatalf("array=%+v err=%v", items, err)
	}

	items, err = service.ParseRecognizeDetail(json.RawMessage(`{}`))
	if err != nil || items == nil || len(items) != 0 {
		t.Fatalf("empty object=%+v err=%v", items, err)
	}
}

func TestFridgeApplyRecognizeEmptyDetail(t *testing.T) {
	db := testutil.SetupTestDB(t)
	userID, familyID := testutil.SeedUserAndFamily(t, db)
	svc := service.NewFridgeService(db, &mockFridgeDispatcher{dispatched: true})

	scan, err := svc.CreateScan(userID, familyID, "fridge/empty.jpg", "https://cdn/empty.jpg")
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.ApplyRecognizeResult(scan.ID, json.RawMessage(`{}`)); err != nil {
		t.Fatal(err)
	}
	got, err := svc.GetScan(familyID, scan.ID)
	if err != nil || got.Status != service.FridgeScanDone {
		t.Fatalf("scan=%+v err=%v", got, err)
	}
	if got.RecognizedItems != "[]" {
		t.Fatalf("recognized_items should be [], got %q", got.RecognizedItems)
	}
	items, err := service.ScanRecognizedItems(got)
	if err != nil || items == nil || len(items) != 0 {
		t.Fatalf("service.ScanRecognizedItems=%+v err=%v", items, err)
	}
}

func TestFridgeApplyRecognizeAndConfirm(t *testing.T) {
	db := testutil.SetupTestDB(t)
	userID, familyID := testutil.SeedUserAndFamily(t, db)
	disp := &mockFridgeDispatcher{dispatched: true}
	svc := service.NewFridgeService(db, disp)

	scan, err := svc.CreateScan(userID, familyID, "fridge/c.jpg", "https://cdn/c.jpg")
	if err != nil {
		t.Fatal(err)
	}

	detail := `{"items":[{"name":"黄瓜","amount":"2根"},{"name":"豆腐"}]}`
	if err := svc.ApplyRecognizeResult(scan.ID, json.RawMessage(detail)); err != nil {
		t.Fatal(err)
	}

	got, err := svc.GetScan(familyID, scan.ID)
	if err != nil || got.Status != service.FridgeScanDone {
		t.Fatalf("scan=%+v err=%v", got, err)
	}

	created, err := svc.ConfirmScan(familyID, userID, scan.ID, []service.FridgeItemInput{
		{Name: "黄瓜", Amount: "2根"},
		{Name: "豆腐", Amount: "1块", ExpiryDate: strPtr("2026-07-01")},
	})
	if err != nil || len(created) != 2 {
		t.Fatalf("created=%+v err=%v", created, err)
	}
	if created[0].Source != service.FridgeSourcePhoto || created[0].ScanID == nil {
		t.Fatalf("item0=%+v", created[0])
	}

	got, _ = svc.GetScan(familyID, scan.ID)
	if got.Status != service.FridgeScanConfirmed || got.ConfirmedAt == nil {
		t.Fatalf("after confirm: %+v", got)
	}

	_, err = svc.ConfirmScan(familyID, userID, scan.ID, []service.FridgeItemInput{{Name: "x"}})
	if !errors.Is(err, service.ErrFridgeScanNotConfirmable) {
		t.Fatalf("want not confirmable, got %v", err)
	}
}

func TestFridgeConfirmRequiresDone(t *testing.T) {
	db := testutil.SetupTestDB(t)
	userID, familyID := testutil.SeedUserAndFamily(t, db)
	svc := service.NewFridgeService(db, &mockFridgeDispatcher{dispatched: true})

	scan := model.FridgeScan{
		FamilyID: familyID, UserID: userID, TaskID: "t1",
		ImageKey: "k", ImageURL: "u", Status: service.FridgeScanProcessing,
	}
	db.Create(&scan)

	_, err := svc.ConfirmScan(familyID, userID, scan.ID, []service.FridgeItemInput{{Name: "a"}})
	if !errors.Is(err, service.ErrFridgeScanNotConfirmable) {
		t.Fatalf("got %v", err)
	}
}

func strPtr(s string) *string { return &s }
