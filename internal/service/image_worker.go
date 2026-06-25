// Package service - 树莓派图片处理网关调度。
//
// 经 ImageWorkerHub WebSocket 派发 compress（菜谱封面压缩）与 recognize（冰箱识别）任务，
// 接收网关回传结果并回调 FridgeRecognizer 或更新菜谱封面。
package service

import (
	"encoding/json"
	"fmt"
	"log"
	"path/filepath"
	"strings"

	"recipe-server/internal/model"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// FridgeRecognizer 冰箱拍照识别结果回调。
type FridgeRecognizer interface {
	ApplyRecognizeResult(scanID uint64, detail json.RawMessage) error
	ApplyRecognizeFailure(scanID uint64, errMsg string) error
}

// ImageWorkerService 向树莓派网关派发图片任务并处理结果。
type ImageWorkerService struct {
	db     *gorm.DB
	hub    *ImageWorkerHub
	fridge FridgeRecognizer
}

func NewImageWorkerService(db *gorm.DB, hub *ImageWorkerHub) *ImageWorkerService {
	s := &ImageWorkerService{db: db, hub: hub}
	hub.onResult = s.handleTaskResult
	return s
}

// SetFridgeRecognizer 注入冰箱识别结果处理器（避免 service 包循环依赖）。
func (s *ImageWorkerService) SetFridgeRecognizer(fr FridgeRecognizer) {
	s.fridge = fr
}

func (s *ImageWorkerService) DispatchCompress(ossKey, ossURL string, recipeID, familyID uint64) {
	if s.hub == nil || !s.hub.IsConnected() {
		log.Printf("[ImageWorker] skip compress (offline): %s", ossKey)
		return
	}
	task := map[string]any{
		"type":    "task",
		"task_id": uuid.NewString(),
		"action":  "compress",
		"oss_key": ossKey,
		"oss_url": ossURL,
	}
	if recipeID > 0 {
		meta := map[string]any{"recipe_id": recipeID}
		if familyID > 0 {
			meta["family_id"] = familyID
		}
		task["meta"] = meta
	}
	if !s.hub.SendTask(task) {
		log.Printf("[ImageWorker] failed to dispatch compress: %s", ossKey)
	}
}

func (s *ImageWorkerService) DispatchRecognize(ossKey, ossURL string, recipeID uint64) {
	if s.hub == nil || !s.hub.IsConnected() {
		log.Printf("[ImageWorker] skip recognize (offline): %s", ossKey)
		return
	}
	task := map[string]any{
		"type":    "task",
		"task_id": uuid.NewString(),
		"action":  "recognize",
		"oss_key": ossKey,
		"oss_url": ossURL,
		"meta":    map[string]any{"recipe_id": recipeID},
	}
	if !s.hub.SendTask(task) {
		log.Printf("[ImageWorker] failed to dispatch recognize: %s", ossKey)
	}
}

func (s *ImageWorkerService) IsWorkerConnected() bool {
	return s.hub != nil && s.hub.IsConnected()
}

// DispatchFridgeRecognize 派发冰箱食材识别任务，taskID 须与 fridge_scans.task_id 一致。
func (s *ImageWorkerService) DispatchFridgeRecognize(scanID uint64, taskID, ossKey, ossURL string) bool {
	if s.hub == nil || !s.hub.IsConnected() {
		log.Printf("[ImageWorker] skip fridge recognize (offline): scan=%d", scanID)
		return false
	}
	if taskID == "" {
		taskID = uuid.NewString()
	}
	task := map[string]any{
		"type":    "task",
		"task_id": taskID,
		"action":  "recognize",
		"oss_key": ossKey,
		"oss_url": ossURL,
		"meta":    map[string]any{"scope": "fridge", "scan_id": scanID},
	}
	if !s.hub.SendTask(task) {
		log.Printf("[ImageWorker] failed to dispatch fridge recognize: scan=%d", scanID)
		return false
	}
	return true
}

type taskMeta struct {
	Scope    string `json:"scope"`
	ScanID   uint64 `json:"scan_id"`
	RecipeID uint64 `json:"recipe_id"`
	FamilyID uint64 `json:"family_id"`
}

func parseTaskMeta(raw json.RawMessage) taskMeta {
	var m taskMeta
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &m)
	}
	return m
}

func taskResultOK(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "ok", "success", "done":
		return true
	default:
		return false
	}
}

// lookupFridgeScanID 根据 meta 或 task_id 关联冰箱识别任务（树莓派回传常不带 meta）。
func (s *ImageWorkerService) lookupFridgeScanID(taskID string, meta taskMeta) uint64 {
	if meta.Scope == "fridge" && meta.ScanID > 0 {
		return meta.ScanID
	}
	if taskID == "" {
		return 0
	}
	var scan model.FridgeScan
	if err := s.db.Where("task_id = ?", taskID).First(&scan).Error; err != nil {
		return 0
	}
	return scan.ID
}

func (s *ImageWorkerService) handleTaskResult(data []byte) {
	var msg struct {
		Type     string          `json:"type"`
		TaskID   string          `json:"task_id"`
		Status   string          `json:"status"`
		Action   string          `json:"action"`
		OssKey   string          `json:"oss_key"`
		ErrorMsg string          `json:"error_msg"`
		Detail   json.RawMessage `json:"detail"`
		Meta     json.RawMessage `json:"meta"`
	}
	if err := json.Unmarshal(data, &msg); err != nil {
		log.Printf("[ImageWorker] bad task_result: %v", err)
		return
	}
	meta := parseTaskMeta(msg.Meta)
	scanID := s.lookupFridgeScanID(msg.TaskID, meta)

	if !taskResultOK(msg.Status) {
		log.Printf("[ImageWorker] task %s error: %s", msg.TaskID, msg.ErrorMsg)
		if scanID > 0 && s.fridge != nil {
			errMsg := msg.ErrorMsg
			if errMsg == "" {
				errMsg = "识别失败"
			}
			_ = s.fridge.ApplyRecognizeFailure(scanID, errMsg)
		}
		return
	}
	switch msg.Action {
	case "compress":
		s.handleCompressResult(msg.OssKey, meta.RecipeID, meta.FamilyID, msg.Detail)
	case "recognize":
		if scanID > 0 {
			s.handleFridgeRecognizeResult(scanID, msg.Detail)
		} else {
			s.handleRecognizeResult(meta.RecipeID, msg.Detail)
		}
	default:
		// 树莓派可能省略 action，按 task_id 关联到冰箱 scan
		if scanID > 0 && len(msg.Detail) > 0 {
			s.handleFridgeRecognizeResult(scanID, msg.Detail)
		} else if meta.RecipeID > 0 {
			s.handleRecognizeResult(meta.RecipeID, msg.Detail)
		} else {
			log.Printf("[ImageWorker] unhandled task_result action=%q task_id=%s", msg.Action, msg.TaskID)
		}
	}
}

func (s *ImageWorkerService) handleFridgeRecognizeResult(scanID uint64, detail json.RawMessage) {
	if s.fridge == nil {
		log.Printf("[ImageWorker] fridge recognizer not set scan=%d", scanID)
		return
	}
	if err := s.fridge.ApplyRecognizeResult(scanID, detail); err != nil {
		log.Printf("[ImageWorker] fridge recognize scan=%d: %v", scanID, err)
	}
}

func (s *ImageWorkerService) handleCompressResult(oldKey string, recipeID, familyID uint64, detail json.RawMessage) {
	var d struct {
		Skipped   bool   `json:"skipped"`
		NewOssKey string `json:"new_oss_key"`
	}
	if err := json.Unmarshal(detail, &d); err != nil {
		log.Printf("[ImageWorker] compress detail parse: %v", err)
		return
	}
	if d.Skipped || d.NewOssKey == "" || d.NewOssKey == oldKey {
		return
	}
	if err := s.updateRecipeImageKey(recipeID, familyID, oldKey, d.NewOssKey); err != nil {
		log.Printf("[ImageWorker] update image key: %v", err)
	}
}

func (s *ImageWorkerService) handleRecognizeResult(recipeID uint64, detail json.RawMessage) {
	if recipeID == 0 {
		return
	}
	var d struct {
		Ingredients []string `json:"ingredients"`
	}
	if err := json.Unmarshal(detail, &d); err != nil {
		log.Printf("[ImageWorker] recognize detail parse: %v", err)
		return
	}
	if len(d.Ingredients) == 0 {
		return
	}
	b, err := json.Marshal(d.Ingredients)
	if err != nil {
		return
	}
	if err := s.db.Model(&model.Recipe{}).Where("id = ?", recipeID).Update("ingredients", string(b)).Error; err != nil {
		log.Printf("[ImageWorker] update ingredients recipe=%d: %v", recipeID, err)
	}
}

func (s *ImageWorkerService) updateRecipeImageKey(recipeID, familyID uint64, oldKey, newKey string) error {
	newURL, err := BuildObjectURL(newKey)
	if err != nil {
		return err
	}
	q := s.db.Model(&model.Recipe{})
	if recipeID > 0 {
		q = q.Where("id = ?", recipeID)
		if familyID > 0 {
			q = q.Where("family_id = ?", familyID)
		}
	} else {
		q = q.Where("image_key = ?", oldKey)
	}
	res := q.Updates(map[string]any{
		"image_key": newKey,
		"cover_url": newURL,
	})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		log.Printf("[ImageWorker] no recipe matched old_key=%s recipe_id=%d", oldKey, recipeID)
		return nil
	}
	if oldKey != "" && oldKey != newKey {
		if err := DeleteObject(oldKey); err != nil {
			log.Printf("[ImageWorker] delete old oss key %s: %v", oldKey, err)
		}
	}
	log.Printf("[ImageWorker] updated image_key %s -> %s", oldKey, newKey)
	return nil
}

// ParseRecipeIDForm 从 multipart 表单解析可选 recipe_id。
func ParseRecipeIDForm(raw string) uint64 {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0
	}
	var id uint64
	_, _ = fmt.Sscan(raw, &id)
	return id
}

// KeyExtensionChanged 判断压缩后 key 后缀是否变化。
func KeyExtensionChanged(oldKey, newKey string) bool {
	return strings.ToLower(filepath.Ext(oldKey)) != strings.ToLower(filepath.Ext(newKey))
}
