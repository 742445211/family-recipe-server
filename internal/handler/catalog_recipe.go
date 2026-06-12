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

// CatalogRecipeHandler 全局菜谱库接口。
type CatalogRecipeHandler struct {
	svc *service.CatalogRecipeService
}

func NewCatalogRecipeHandler(svc *service.CatalogRecipeService) *CatalogRecipeHandler {
	return &CatalogRecipeHandler{svc: svc}
}

func catalogRecipeDisabled(c *gin.Context) bool {
	if config.AppConfig == nil || !config.AppConfig.CatalogRecipeEnabled() {
		c.JSON(http.StatusForbidden, gin.H{"code": 403, "msg": "菜谱搜索生成功能未开启"})
		return true
	}
	return false
}

type catalogLookupReq struct {
	Name       string `json:"name" binding:"required"`
	NewVariant bool   `json:"new_variant"`
}

// Lookup POST /api/catalog-recipes/lookup
func (h *CatalogRecipeHandler) Lookup(c *gin.Context) {
	if catalogRecipeDisabled(c) {
		return
	}
	var req catalogLookupReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "参数错误: 请提供菜名"})
		return
	}
	userID := middleware.GetUserID(c)
	result, err := h.svc.LookupOrGenerate(c.Request.Context(), userID, req.Name, service.CatalogLookupOpts{
		NewVariant: req.NewVariant,
	})
	if errors.Is(err, service.ErrCatalogRateLimitExceeded) {
		c.JSON(http.StatusTooManyRequests, gin.H{
			"code": 429,
			"msg":  "菜谱生成次数已达上限，请稍后再试",
			"data": result,
		})
		return
	}
	if errors.Is(err, service.ErrCatalogNameEmpty) {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok", "data": result})
}

// Get GET /api/catalog-recipes/:id
func (h *CatalogRecipeHandler) Get(c *gin.Context) {
	if catalogRecipeDisabled(c) {
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	rec, err := h.svc.GetByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "菜谱不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok", "data": rec})
}

// Use POST /api/catalog-recipes/:id/use
func (h *CatalogRecipeHandler) Use(c *gin.Context) {
	if catalogRecipeDisabled(c) {
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if _, err := h.svc.GetByID(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "菜谱不存在"})
		return
	}
	if err := h.svc.IncrementUseCount(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "操作失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok"})
}
