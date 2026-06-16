package handler

import (
	"errors"
	"net/http"
	"strconv"

	"recipe-server/config"
	"recipe-server/internal/middleware"
	"recipe-server/internal/service"

	"github.com/gin-gonic/gin"
)

// FridgeHandler 冰箱食材接口。
type FridgeHandler struct {
	svc *service.FridgeService
}

func NewFridgeHandler(svc *service.FridgeService) *FridgeHandler {
	return &FridgeHandler{svc: svc}
}

func fridgeDisabled(c *gin.Context) bool {
	if config.AppConfig == nil || !config.AppConfig.FridgeEnabled() {
		c.JSON(http.StatusForbidden, gin.H{"code": 403, "msg": "冰箱功能未开启"})
		return true
	}
	return false
}

func requireFamilyID(c *gin.Context) (uint64, bool) {
	fid := middleware.GetFamilyID(c)
	if fid == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "请先加入家庭"})
		return 0, false
	}
	return fid, true
}

type fridgeCreateItemsReq struct {
	Name       string              `json:"name"`
	Amount     string              `json:"amount"`
	ExpiryDate *string             `json:"expiry_date"`
	Note       string              `json:"note"`
	Items      []service.FridgeItemInput `json:"items"`
}

func (r fridgeCreateItemsReq) toInputs() []service.FridgeItemInput {
	if len(r.Items) > 0 {
		return r.Items
	}
	return []service.FridgeItemInput{{
		Name: r.Name, Amount: r.Amount, ExpiryDate: r.ExpiryDate, Note: r.Note,
	}}
}

// ListItems GET /api/fridge/items
func (h *FridgeHandler) ListItems(c *gin.Context) {
	if fridgeDisabled(c) {
		return
	}
	familyID, ok := requireFamilyID(c)
	if !ok {
		return
	}
	items, err := h.svc.ListItems(familyID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok", "data": items})
}

// CreateItems POST /api/fridge/items
func (h *FridgeHandler) CreateItems(c *gin.Context) {
	if fridgeDisabled(c) {
		return
	}
	familyID, ok := requireFamilyID(c)
	if !ok {
		return
	}
	var req fridgeCreateItemsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "参数错误"})
		return
	}
	inputs := req.toInputs()
	if len(inputs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "请提供食材信息"})
		return
	}
	userID := middleware.GetUserID(c)
	if len(inputs) == 1 {
		item, err := h.svc.CreateItem(familyID, userID, inputs[0])
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok", "data": item})
		return
	}
	created, err := h.svc.CreateItemsBatch(familyID, userID, inputs)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok", "data": created})
}

// UpdateItem PUT /api/fridge/items/:id
func (h *FridgeHandler) UpdateItem(c *gin.Context) {
	if fridgeDisabled(c) {
		return
	}
	familyID, ok := requireFamilyID(c)
	if !ok {
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var in service.FridgeItemInput
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "参数错误"})
		return
	}
	item, err := h.svc.UpdateItem(familyID, id, in)
	if errors.Is(err, service.ErrFridgeItemNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": err.Error()})
		return
	}
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok", "data": item})
}

// DeleteItem DELETE /api/fridge/items/:id
func (h *FridgeHandler) DeleteItem(c *gin.Context) {
	if fridgeDisabled(c) {
		return
	}
	familyID, ok := requireFamilyID(c)
	if !ok {
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.svc.DeleteItem(familyID, id); errors.Is(err, service.ErrFridgeItemNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": err.Error()})
		return
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok"})
}

// CreateScan POST /api/fridge/scans
func (h *FridgeHandler) CreateScan(c *gin.Context) {
	if fridgeDisabled(c) {
		return
	}
	familyID, ok := requireFamilyID(c)
	if !ok {
		return
	}
	userID := middleware.GetUserID(c)

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "请选择图片"})
		return
	}
	defer file.Close()

	key, url, err := service.SaveImage(file, header)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
		return
	}

	scan, err := h.svc.CreateScan(userID, familyID, key, url)
	if errors.Is(err, service.ErrFridgeWorkerOffline) {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"code": 503, "msg": err.Error(),
			"data": gin.H{"scan": scan, "worker_offline": true},
		})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok", "data": scan})
}

// GetScan GET /api/fridge/scans/:id
func (h *FridgeHandler) GetScan(c *gin.Context) {
	if fridgeDisabled(c) {
		return
	}
	familyID, ok := requireFamilyID(c)
	if !ok {
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	scan, err := h.svc.GetScan(familyID, id)
	if errors.Is(err, service.ErrFridgeScanNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": err.Error()})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
		return
	}
	items, _ := service.ScanRecognizedItems(scan)
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok", "data": gin.H{
		"id":                scan.ID,
		"status":            scan.Status,
		"image_url":         scan.ImageURL,
		"error_msg":         scan.ErrorMsg,
		"recognized_items":  items,
		"confirmed_at":      scan.ConfirmedAt,
	}})
}

type fridgeConfirmReq struct {
	Items []service.FridgeItemInput `json:"items" binding:"required"`
}

// ConfirmScan POST /api/fridge/scans/:id/confirm
func (h *FridgeHandler) ConfirmScan(c *gin.Context) {
	if fridgeDisabled(c) {
		return
	}
	familyID, ok := requireFamilyID(c)
	if !ok {
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req fridgeConfirmReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "参数错误"})
		return
	}
	userID := middleware.GetUserID(c)
	created, err := h.svc.ConfirmScan(familyID, userID, id, req.Items)
	if errors.Is(err, service.ErrFridgeScanNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": err.Error()})
		return
	}
	if errors.Is(err, service.ErrFridgeScanNotConfirmable) || errors.Is(err, service.ErrFridgeConfirmEmpty) {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok", "data": created})
}
