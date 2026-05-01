package handler

import (
	"net/http"
	"strconv"

	"recipe-server/internal/middleware"
	"recipe-server/internal/model"
	"recipe-server/internal/service"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type MenuHandler struct {
	svc *service.MenuService
	db  *gorm.DB
}

func NewMenuHandler(db *gorm.DB) *MenuHandler {
	return &MenuHandler{svc: service.NewMenuService(db), db: db}
}

func (h *MenuHandler) Create(c *gin.Context) {
	var m model.Menu
	if err := c.ShouldBindJSON(&m); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "参数错误"})
		return
	}
	m.FamilyID = middleware.GetFamilyID(c)
	m.CreatorID = middleware.GetUserID(c)

	if err := h.svc.Create(&m); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "创建失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok", "data": m})
}

func (h *MenuHandler) Get(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	m, err := h.svc.GetByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "菜单不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok", "data": m})
}

func (h *MenuHandler) List(c *gin.Context) {
	familyID := middleware.GetFamilyID(c)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	menus, total, err := h.svc.List(familyID, page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "查询失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"list":      menus,
			"total":     total,
			"page":      page,
			"page_size": pageSize,
		},
	})
}

type addItemReq struct {
	RecipeID uint64 `json:"recipe_id" binding:"required"`
	Quantity int    `json:"quantity"`
	Note     string `json:"note"`
}

func (h *MenuHandler) AddItem(c *gin.Context) {
	menuID, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req addItemReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "参数错误: recipe_id必填"})
		return
	}
	if req.Quantity <= 0 {
		req.Quantity = 1
	}

	item, err := h.svc.AddItem(menuID, req.RecipeID, middleware.GetUserID(c), req.Quantity, req.Note)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok", "data": item})
}

func (h *MenuHandler) RemoveItem(c *gin.Context) {
	menuID, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	itemID, _ := strconv.ParseUint(c.Param("item_id"), 10, 64)
	if err := h.svc.RemoveItem(itemID, middleware.GetUserID(c)); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok", "data": gin.H{"menu_id": menuID}})
}

func (h *MenuHandler) Confirm(c *gin.Context) {
	menuID, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.svc.ConfirmMenu(menuID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "确认失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok"})
}
