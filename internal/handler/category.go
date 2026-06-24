// Package handler - 菜谱分类 HTTP 接口。
//
// GET /api/categories 返回当前家庭分类；GET /api/categories/public 返回公开菜谱分类（无需登录）。
package handler

import (
	"net/http"

	"recipe-server/internal/middleware"
	"recipe-server/internal/service"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// CategoryHandler 菜谱分类接口。
type CategoryHandler struct {
	svc *service.CategoryService
}

func NewCategoryHandler(db *gorm.DB) *CategoryHandler {
	return &CategoryHandler{svc: service.NewCategoryService(db)}
}

// List GET /api/categories — 当前家庭的菜谱分类列表。
func (h *CategoryHandler) List(c *gin.Context) {
	familyID := middleware.GetFamilyID(c)
	if familyID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "请先加入家庭"})
		return
	}
	if err := h.svc.SyncFromRecipes(familyID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "同步分类失败"})
		return
	}
	cats, err := h.svc.List(familyID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "查询失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": cats})
}

// ListPublic GET /api/categories/public — 公开菜谱中出现过的分类（无需登录）。
func (h *CategoryHandler) ListPublic(c *gin.Context) {
	names, err := h.svc.ListPublicNames()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "查询失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": names})
}
