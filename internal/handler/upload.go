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

// UploadHandler 文件上传处理器。
type UploadHandler struct {
	DB          *gorm.DB
	ImageWorker *service.ImageWorkerService
}

// Upload 上传图片文件接口。
func (h *UploadHandler) Upload(c *gin.Context) {
	userID := middleware.GetUserID(c)
	familyID := middleware.GetFamilyID(c)

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "请选择文件"})
		return
	}
	defer file.Close()

	key, url, err := service.UploadImage(middleware.GetOpenID(c), file, header)
	if err != nil {
		if respondSecCheck(c, err) {
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
		return
	}

	var recipeID uint64
	if raw := c.PostForm("recipe_id"); raw != "" {
		id, parseErr := strconv.ParseUint(raw, 10, 64)
		if parseErr != nil || id == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "无效的菜谱 ID"})
			return
		}
		if familyID == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "请先加入家庭"})
			return
		}
		var recipe model.Recipe
		q := h.DB.Where("id = ? AND family_id = ?", id, familyID)
		if err := q.First(&recipe).Error; err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "菜谱不存在或无权操作"})
			return
		}
		recipeID = id
	}

	if h.ImageWorker != nil {
		h.ImageWorker.DispatchCompress(key, url, recipeID, familyID)
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok", "data": gin.H{
		"key": key,
		"url": url,
	}})
	_ = userID
}
