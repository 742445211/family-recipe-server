// Package handler 提供 HTTP 请求处理器（Gin handlers）。
//
// 本文件 (favorite.go) 负责收藏相关接口：
//   - 收藏菜谱（幂等）
//   - 取消收藏（软删除）
//   - 查看收藏列表
package handler

import (
	"errors"
	"net/http"
	"strconv"

	"recipe-server/internal/middleware"
	"recipe-server/internal/model"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// FavoriteHandler 处理收藏相关的 HTTP 请求（添加、移除、列表）。
type FavoriteHandler struct {
	db *gorm.DB // 数据库连接
}

// NewFavoriteHandler 创建收藏处理器。
func NewFavoriteHandler(db *gorm.DB) *FavoriteHandler {
	return &FavoriteHandler{db: db}
}

// Add 收藏一道菜谱接口。
//
// 路由：POST /api/favorites/:id（需认证）
//
// 功能：
//   将指定菜谱添加到当前用户的收藏夹。
//   使用 FirstOrCreate 实现幂等：重复收藏不报错，不重复插入。
//
// 路径参数：
//   - id: 菜谱 ID
//
// 响应：
//   - 成功：{"code":0, "msg":"ok"}
//   - 失败：{"code":500, "msg":"收藏失败"}
func (h *FavoriteHandler) Add(c *gin.Context) {
	// 从 URL 路径参数解析菜谱 ID
	recipeID, _ := strconv.ParseUint(c.Param("id"), 10, 64)

	userID := middleware.GetUserID(c)
	familyID := middleware.GetFamilyID(c)
	if familyID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "请先加入家庭"})
		return
	}
	if err := h.db.Where("id = ? AND (family_id = ? OR is_public = ?)", recipeID, familyID, true).
		First(&model.Recipe{}).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "菜谱不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "收藏失败"})
		return
	}
	// 使用 FirstOrCreate 实现幂等收藏：
	//   先按 user_id+recipe_id 查找，存在则返回已有记录，不存在则插入新记录
	fav := model.Favorite{
		UserID:   userID,
		RecipeID: recipeID,
	}
	if err := h.db.Where(model.Favorite{UserID: userID, RecipeID: recipeID}).
		FirstOrCreate(&fav).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "收藏失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok"})
}

// Remove 取消收藏一道菜谱接口。
//
// 路由：DELETE /api/favorites/:id（需认证）
//
// 功能：
//   从当前用户的收藏夹中移除指定菜谱（GORM 软删除，设置 deleted_at 时间戳）。
//
// 路径参数：
//   - id: 菜谱 ID
//
// 响应：
//   - 成功：{"code":0, "msg":"ok"}
func (h *FavoriteHandler) Remove(c *gin.Context) {
	// 从 URL 路径参数解析菜谱 ID
	recipeID, _ := strconv.ParseUint(c.Param("id"), 10, 64)

	// 从 JWT 上下文中获取当前用户 ID
	userID := middleware.GetUserID(c)

	// 按 user_id + recipe_id 软删除收藏记录
	// GORM 的 Delete 在有 DeletedAt 字段时自动执行软删除
	h.db.Where("user_id = ? AND recipe_id = ?", userID, recipeID).Delete(&model.Favorite{})

	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok"})
}

// List 获取当前用户的收藏列表接口。
//
// 路由：GET /api/favorites（需认证）
//
// 功能：
//   返回当前用户收藏的所有菜谱，附带菜谱详情，按收藏时间倒序排列。
//
// 响应：
//   - 成功：{"code":0, "data":[{"id":1,"user_id":2,"recipe_id":5,"recipe":{...},...}]}
func (h *FavoriteHandler) List(c *gin.Context) {
	// 从 JWT 上下文中获取当前用户 ID
	userID := middleware.GetUserID(c)

	var favs []model.Favorite
	h.db.Where("user_id = ?", userID).
		Preload("Recipe").        // 预加载菜谱信息，避免 N+1 查询
		Order("created_at DESC"). // 按收藏时间倒序，最新收藏在前
		Find(&favs)

	c.JSON(http.StatusOK, gin.H{"code": 0, "data": favs})
}
