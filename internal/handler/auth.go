package handler

import (
	"net/http"

	"recipe-server/config"
	"recipe-server/internal/middleware"
	"recipe-server/internal/model"
	"recipe-server/internal/service"
	jwtPkg "recipe-server/pkg/jwt"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type AuthHandler struct {
	db *gorm.DB
}

func NewAuthHandler(db *gorm.DB) *AuthHandler {
	return &AuthHandler{db: db}
}

type loginReq struct {
	Code     string `json:"code" binding:"required"`
	Nickname string `json:"nickname"`
	Avatar   string `json:"avatar_url"`
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req loginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "参数错误: code必填"})
		return
	}

	// 用 code 换 openid
	session, err := service.Code2Session(req.Code)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
		return
	}

	// 创建或更新用户
	var user model.User
	result := h.db.Where("openid = ?", session.OpenID).First(&user)
	if result.Error != nil {
		user = model.User{
			OpenID:    session.OpenID,
			UnionID:   session.UnionID,
			Nickname:  req.Nickname,
			AvatarURL: req.Avatar,
		}
		if err := h.db.Create(&user).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "创建用户失败"})
			return
		}
	} else {
		if req.Nickname != "" {
			user.Nickname = req.Nickname
		}
		if req.Avatar != "" {
			user.AvatarURL = req.Avatar
		}
		h.db.Save(&user)
	}

	// 签发 JWT
	familyID := uint64(0)
	if user.CurrentFamilyID != nil {
		familyID = *user.CurrentFamilyID
	}
	token, err := jwtPkg.Generate(config.AppConfig.JWT.Secret, config.AppConfig.JWT.ExpireHours, user.ID, user.OpenID, familyID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "生成token失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"msg":     "ok",
		"data": gin.H{
			"token":    token,
			"user_id":  user.ID,
			"openid":   user.OpenID,
			"nickname": user.Nickname,
			"avatar":   user.AvatarURL,
		},
	})
}

func (h *AuthHandler) GetProfile(c *gin.Context) {
	userID := middleware.GetUserID(c)
	var user model.User
	if err := h.db.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "用户不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok", "data": user})
}

type updateProfileReq struct {
	Nickname        string  `json:"nickname"`
	AvatarURL       string  `json:"avatar_url"`
	CurrentFamilyID *uint64 `json:"current_family_id"`
}

func (h *AuthHandler) UpdateProfile(c *gin.Context) {
	userID := middleware.GetUserID(c)
	var req updateProfileReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "参数错误"})
		return
	}
	updates := map[string]any{}
	if req.Nickname != "" {
		updates["nickname"] = req.Nickname
	}
	if req.AvatarURL != "" {
		updates["avatar_url"] = req.AvatarURL
	}
	if req.CurrentFamilyID != nil {
		updates["current_family_id"] = *req.CurrentFamilyID
	}
	if err := h.db.Model(&model.User{}).Where("id = ?", userID).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "更新失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok"})
}
