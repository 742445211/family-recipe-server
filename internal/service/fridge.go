package service

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"recipe-server/internal/model"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	FridgeSourceManual = "manual"
	FridgeSourcePhoto  = "photo"

	FridgeScanPending    = "pending"
	FridgeScanProcessing = "processing"
	FridgeScanDone       = "done"
	FridgeScanFailed     = "failed"
	FridgeScanConfirmed  = "confirmed"
)

var (
	ErrFridgeNoFamily          = errors.New("请先加入家庭")
	ErrFridgeItemNotFound      = errors.New("食材不存在")
	ErrFridgeScanNotFound      = errors.New("识别任务不存在")
	ErrFridgeWorkerOffline     = errors.New("图片识别服务离线")
	ErrFridgeScanNotConfirmable = errors.New("识别任务不可确认")
	ErrFridgeConfirmEmpty      = errors.New("请至少选择一条食材")
	ErrFridgeScanNotRetryable  = errors.New("识别任务不可重试")
)

// FridgeItemInput 创建/更新/确认食材输入。
type FridgeItemInput struct {
	Name       string  `json:"name"`
	Amount     string  `json:"amount"`
	ExpiryDate *string `json:"expiry_date"`
	Note       string  `json:"note"`
}

// FridgeImageDispatcher 向树莓派派发冰箱识别任务。
type FridgeImageDispatcher interface {
	DispatchFridgeRecognize(scanID uint64, taskID, ossKey, ossURL string) bool
	IsWorkerConnected() bool
}

// FridgeService 冰箱库存与拍照识别业务。
type FridgeService struct {
	db   *gorm.DB
	disp FridgeImageDispatcher
}

func NewFridgeService(db *gorm.DB, disp FridgeImageDispatcher) *FridgeService {
	return &FridgeService{db: db, disp: disp}
}

func (s *FridgeService) ListItems(familyID uint64) ([]model.FridgeItem, error) {
	var items []model.FridgeItem
	err := s.db.Where("family_id = ?", familyID).
		Order("CASE WHEN expiry_date IS NULL THEN 1 ELSE 0 END, expiry_date ASC, updated_at DESC").
		Find(&items).Error
	return items, err
}

func (s *FridgeService) CreateItem(familyID, userID uint64, in FridgeItemInput) (*model.FridgeItem, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return nil, errors.New("食材名称不能为空")
	}
	item := model.FridgeItem{
		FamilyID:   familyID,
		Name:       name,
		Amount:     strings.TrimSpace(in.Amount),
		ExpiryDate: in.ExpiryDate,
		Note:       strings.TrimSpace(in.Note),
		Source:     FridgeSourceManual,
		AddedBy:    userID,
	}
	if err := s.db.Create(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *FridgeService) CreateItemsBatch(familyID, userID uint64, inputs []FridgeItemInput) ([]model.FridgeItem, error) {
	out := make([]model.FridgeItem, 0, len(inputs))
	for _, in := range inputs {
		item, err := s.CreateItem(familyID, userID, in)
		if err != nil {
			return nil, err
		}
		out = append(out, *item)
	}
	return out, nil
}

func (s *FridgeService) UpdateItem(familyID, itemID uint64, in FridgeItemInput) (*model.FridgeItem, error) {
	var item model.FridgeItem
	if err := s.db.Where("id = ? AND family_id = ?", itemID, familyID).First(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrFridgeItemNotFound
		}
		return nil, err
	}
	if name := strings.TrimSpace(in.Name); name != "" {
		item.Name = name
	}
	item.Amount = strings.TrimSpace(in.Amount)
	item.Note = strings.TrimSpace(in.Note)
	if in.ExpiryDate != nil {
		item.ExpiryDate = in.ExpiryDate
	}
	if err := s.db.Save(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *FridgeService) DeleteItem(familyID, itemID uint64) error {
	res := s.db.Where("id = ? AND family_id = ?", itemID, familyID).Delete(&model.FridgeItem{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrFridgeItemNotFound
	}
	return nil
}

func (s *FridgeService) CreateScan(userID, familyID uint64, imageKey, imageURL string) (*model.FridgeScan, error) {
	taskID := uuid.NewString()
	scan := model.FridgeScan{
		FamilyID: familyID,
		UserID:   userID,
		TaskID:   taskID,
		ImageKey: imageKey,
		ImageURL: imageURL,
		Status:   FridgeScanPending,
	}
	if err := s.db.Create(&scan).Error; err != nil {
		return nil, err
	}

	if s.disp == nil {
		scan.Status = FridgeScanFailed
		scan.ErrorMsg = ErrFridgeWorkerOffline.Error()
		_ = s.db.Save(&scan).Error
		return &scan, ErrFridgeWorkerOffline
	}

	sent := s.disp.DispatchFridgeRecognize(scan.ID, scan.TaskID, imageKey, imageURL)
	if !sent {
		scan.Status = FridgeScanFailed
		scan.ErrorMsg = ErrFridgeWorkerOffline.Error()
		_ = s.db.Save(&scan).Error
		return &scan, ErrFridgeWorkerOffline
	}
	scan.Status = FridgeScanProcessing
	if err := s.db.Save(&scan).Error; err != nil {
		return nil, err
	}
	return &scan, nil
}

// RetryScan 对 processing/failed 的识别任务重新派发（同一 task_id）。
func (s *FridgeService) RetryScan(familyID, scanID uint64) (*model.FridgeScan, error) {
	scan, err := s.GetScan(familyID, scanID)
	if err != nil {
		return nil, err
	}
	if scan.Status == FridgeScanConfirmed || scan.Status == FridgeScanDone {
		return nil, ErrFridgeScanNotRetryable
	}
	if s.disp == nil || !s.disp.IsWorkerConnected() {
		return nil, ErrFridgeWorkerOffline
	}
	if !s.disp.DispatchFridgeRecognize(scan.ID, scan.TaskID, scan.ImageKey, scan.ImageURL) {
		return nil, ErrFridgeWorkerOffline
	}
	scan.Status = FridgeScanProcessing
	scan.ErrorMsg = ""
	if err := s.db.Save(scan).Error; err != nil {
		return nil, err
	}
	return scan, nil
}

func (s *FridgeService) GetScan(familyID, scanID uint64) (*model.FridgeScan, error) {
	var scan model.FridgeScan
	if err := s.db.Where("id = ? AND family_id = ?", scanID, familyID).First(&scan).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrFridgeScanNotFound
		}
		return nil, err
	}
	return &scan, nil
}

func (s *FridgeService) ApplyRecognizeResult(scanID uint64, detail json.RawMessage) error {
	items, err := ParseRecognizeDetail(detail)
	if err != nil {
		return err
	}
	b, err := json.Marshal(items)
	if err != nil {
		return err
	}
	res := s.db.Model(&model.FridgeScan{}).Where("id = ? AND status IN ?", scanID,
		[]string{FridgeScanPending, FridgeScanProcessing}).
		Updates(map[string]any{
			"status":            FridgeScanDone,
			"recognized_items":  string(b),
			"error_msg":         "",
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		var scan model.FridgeScan
		if err := s.db.First(&scan, scanID).Error; err != nil {
			return err
		}
		if scan.Status == FridgeScanDone || scan.Status == FridgeScanConfirmed {
			return nil
		}
		return ErrFridgeScanNotFound
	}
	return nil
}

func (s *FridgeService) ApplyRecognizeFailure(scanID uint64, errMsg string) error {
	return s.db.Model(&model.FridgeScan{}).Where("id = ?", scanID).
		Updates(map[string]any{
			"status":    FridgeScanFailed,
			"error_msg": errMsg,
		}).Error
}

func (s *FridgeService) ConfirmScan(familyID, userID, scanID uint64, inputs []FridgeItemInput) ([]model.FridgeItem, error) {
	if len(inputs) == 0 {
		return nil, ErrFridgeConfirmEmpty
	}
	for _, in := range inputs {
		if strings.TrimSpace(in.Name) == "" {
			return nil, errors.New("食材名称不能为空")
		}
	}

	var created []model.FridgeItem
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var scan model.FridgeScan
		if err := tx.Where("id = ? AND family_id = ?", scanID, familyID).First(&scan).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrFridgeScanNotFound
			}
			return err
		}
		if scan.Status != FridgeScanDone {
			return ErrFridgeScanNotConfirmable
		}

		scanIDRef := scan.ID
		for _, in := range inputs {
			item := model.FridgeItem{
				FamilyID:   familyID,
				Name:       strings.TrimSpace(in.Name),
				Amount:     strings.TrimSpace(in.Amount),
				ExpiryDate: in.ExpiryDate,
				Note:       strings.TrimSpace(in.Note),
				Source:     FridgeSourcePhoto,
				ScanID:     &scanIDRef,
				AddedBy:    userID,
			}
			if err := tx.Create(&item).Error; err != nil {
				return err
			}
			created = append(created, item)
		}

		now := time.Now()
		return tx.Model(&scan).Updates(map[string]any{
			"status":        FridgeScanConfirmed,
			"confirmed_at":  now,
		}).Error
	})
	if err != nil {
		return nil, err
	}
	return created, nil
}

// ParseRecognizeDetail 解析树莓派识别结果（支持 items、ingredients、顶层数组）。
func ParseRecognizeDetail(detail json.RawMessage) ([]FridgeItemInput, error) {
	if len(detail) == 0 {
		return nil, nil
	}
	var structured struct {
		Items []FridgeItemInput `json:"items"`
	}
	if err := json.Unmarshal(detail, &structured); err == nil && len(structured.Items) > 0 {
		return structured.Items, nil
	}
	var legacy struct {
		Ingredients []string `json:"ingredients"`
	}
	if err := json.Unmarshal(detail, &legacy); err == nil && len(legacy.Ingredients) > 0 {
		out := make([]FridgeItemInput, 0, len(legacy.Ingredients))
		for _, name := range legacy.Ingredients {
			n := strings.TrimSpace(name)
			if n != "" {
				out = append(out, FridgeItemInput{Name: n})
			}
		}
		return out, nil
	}
	var names []string
	if err := json.Unmarshal(detail, &names); err == nil && len(names) > 0 {
		out := make([]FridgeItemInput, 0, len(names))
		for _, name := range names {
			n := strings.TrimSpace(name)
			if n != "" {
				out = append(out, FridgeItemInput{Name: n})
			}
		}
		return out, nil
	}
	var objs []FridgeItemInput
	if err := json.Unmarshal(detail, &objs); err == nil && len(objs) > 0 {
		return objs, nil
	}
	return nil, nil
}

// ScanRecognizedItems 将 scan JSON 解析为候选列表。
func ScanRecognizedItems(scan *model.FridgeScan) ([]FridgeItemInput, error) {
	if scan == nil || scan.RecognizedItems == "" || scan.RecognizedItems == "null" {
		return nil, nil
	}
	var items []FridgeItemInput
	if err := json.Unmarshal([]byte(scan.RecognizedItems), &items); err != nil {
		return nil, err
	}
	return items, nil
}
