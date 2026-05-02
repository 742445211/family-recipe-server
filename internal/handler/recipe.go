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

type RecipeHandler struct {
	svc *service.RecipeService
	db  *gorm.DB
}

func NewRecipeHandler(db *gorm.DB) *RecipeHandler {
	return &RecipeHandler{svc: service.NewRecipeService(db), db: db}
}

func (h *RecipeHandler) Create(c *gin.Context) {
	var r model.Recipe
	if err := c.ShouldBindJSON(&r); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "参数错误"})
		return
	}
	r.CreatorID = middleware.GetUserID(c)
	r.FamilyID = middleware.GetFamilyID(c)

	if err := h.svc.Create(&r); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "创建失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok", "data": r})
}

func (h *RecipeHandler) Update(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var r model.Recipe
	if err := c.ShouldBindJSON(&r); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "参数错误"})
		return
	}
	r.ID = id
	if err := h.svc.Update(&r); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "更新失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok"})
}

func (h *RecipeHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.svc.Delete(id, middleware.GetUserID(c)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "删除失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok"})
}

func (h *RecipeHandler) Get(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	r, err := h.svc.GetByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "菜谱不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok", "data": r})
}

func (h *RecipeHandler) List(c *gin.Context) {
	familyID := uint64(0)
	if fid, exists := c.Get("family_id"); exists {
		familyID = fid.(uint64)
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	recipes, total, err := h.svc.List(
		familyID,
		c.Query("keyword"),
		c.Query("category"),
		page, pageSize,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "查询失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"list":      recipes,
			"total":     total,
			"page":      page,
			"page_size": pageSize,
		},
	})
}

func (h *RecipeHandler) Cooked(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.svc.IncrementCookCount(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "操作失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok"})
}
