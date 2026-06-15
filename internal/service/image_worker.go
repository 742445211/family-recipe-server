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

// ImageWorkerService 向树莓派网关派发图片任务并处理结果。
type ImageWorkerService struct {
	db  *gorm.DB
	hub *ImageWorkerHub
}

func NewImageWorkerService(db *gorm.DB, hub *ImageWorkerHub) *ImageWorkerService {
	s := &ImageWorkerService{db: db, hub: hub}
	hub.onResult = s.handleTaskResult
	return s
}

func (s *ImageWorkerService) DispatchCompress(ossKey, ossURL string, recipeID uint64) {
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
		task["meta"] = map[string]any{"recipe_id": recipeID}
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

func (s *ImageWorkerService) handleTaskResult(data []byte) {
	var msg struct {
		Type     string          `json:"type"`
		TaskID   string          `json:"task_id"`
		Status   string          `json:"status"`
		Action   string          `json:"action"`
		OssKey   string          `json:"oss_key"`
		ErrorMsg string          `json:"error_msg"`
		Detail   json.RawMessage `json:"detail"`
		Meta     struct {
			RecipeID uint64 `json:"recipe_id"`
		} `json:"meta"`
	}
	if err := json.Unmarshal(data, &msg); err != nil {
		log.Printf("[ImageWorker] bad task_result: %v", err)
		return
	}
	if msg.Status != "ok" {
		log.Printf("[ImageWorker] task %s error: %s", msg.TaskID, msg.ErrorMsg)
		return
	}
	switch msg.Action {
	case "compress":
		s.handleCompressResult(msg.OssKey, msg.Meta.RecipeID, msg.Detail)
	case "recognize":
		s.handleRecognizeResult(msg.Meta.RecipeID, msg.Detail)
	}
}

func (s *ImageWorkerService) handleCompressResult(oldKey string, recipeID uint64, detail json.RawMessage) {
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
	if err := s.updateRecipeImageKey(recipeID, oldKey, d.NewOssKey); err != nil {
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

func (s *ImageWorkerService) updateRecipeImageKey(recipeID uint64, oldKey, newKey string) error {
	newURL, err := BuildObjectURL(newKey)
	if err != nil {
		return err
	}
	q := s.db.Model(&model.Recipe{})
	if recipeID > 0 {
		q = q.Where("id = ?", recipeID)
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
