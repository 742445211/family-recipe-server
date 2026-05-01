package handler

import (
	"net/http"
	"strconv"

	"recipe-server/internal/middleware"
	"recipe-server/internal/model"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type FavoriteHandler struct {
	db *gorm.DB
}

func NewFavoriteHandler(db *gorm.DB) *FavoriteHandler {
	return &FavoriteHandler{db: db}
}

func (h *FavoriteHandler) Add(c *gin.Context) {
	recipeID, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	userID := middleware.GetUserID(c)

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

func (h *FavoriteHandler) Remove(c *gin.Context) {
	recipeID, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	userID := middleware.GetUserID(c)

	h.db.Where("user_id = ? AND recipe_id = ?", userID, recipeID).Delete(&model.Favorite{})
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok"})
}

func (h *FavoriteHandler) List(c *gin.Context) {
	userID := middleware.GetUserID(c)
	var favs []model.Favorite
	h.db.Where("user_id = ?", userID).
		Preload("Recipe").
		Order("created_at DESC").
		Find(&favs)
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": favs})
}
