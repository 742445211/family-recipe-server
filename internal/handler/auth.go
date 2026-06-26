// Package handler 提供 HTTP 请求处理器（Gin handlers）。
//
// 本文件 (auth.go) 负责用户认证相关接口：
//   - 微信小程序登录（code 换 token）
//   - 获取/更新用户个人信息
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

// AuthHandler 处理用户认证相关请求（登录、用户信息）。
type AuthHandler struct {
	db *gorm.DB // 数据库连接，用于用户表查询和更新
}

// NewAuthHandler 创建认证处理器。
func NewAuthHandler(db *gorm.DB) *AuthHandler {
	return &AuthHandler{db: db}
}

// loginReq 微信登录请求体。
type loginReq struct {
	Code     string `json:"code" binding:"required"` // 微信小程序 wx.login() 返回的临时 code
	Nickname string `json:"nickname"`                // 用户昵称（可选，新用户提供）
	Avatar   string `json:"avatar_url"`              // 头像 URL（可选，新用户提供）
}

// Login 微信小程序登录接口。
//
// 路由：POST /api/auth/login（公开接口）
//
// 功能：
//   1. 用小程序 code 向微信服务器换取 OpenID/SessionKey
//   2. 查找或创建用户记录
//   3. 签发 JWT Token 并返回用户信息
//
// 请求 Body：
//   - code: string (必填) 微信登录 code
//   - nickname: string (可选) 用户昵称
//   - avatar_url: string (可选) 头像 URL
//
// 响应：
//   - 成功：{"code":0, "msg":"ok", "data":{"token":"...","user_id":1,"openid":"...","nickname":"...","avatar":"..."}}
//   - 失败：{"code":400, "msg":"参数错误: code必填"} / {"code":500, "msg":"创建用户失败"} / ...
func (h *AuthHandler) Login(c *gin.Context) {
	var req loginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "参数错误: code必填"})
		return
	}

	// 1. 用小程序 code 换取微信 session（OpenID、UnionID 等）
	session, err := service.Code2Session(req.Code)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "微信登录失败"})
		return
	}

	// 2. 根据 OpenID 查找已有用户，不存在则创建新用户
	var user model.User
	result := h.db.Where("openid = ?", session.OpenID).First(&user)
	if result.Error != nil {
		// 新用户：创建记录（写入 OpenID、昵称、头像）
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
		// 老用户：更新昵称和头像（如果提供了新值，非空才覆盖）
		if req.Nickname != "" {
			user.Nickname = req.Nickname
		}
		if req.Avatar != "" {
			user.AvatarURL = req.Avatar
		}
		h.db.Save(&user) // Save 执行全量更新（包含 updated_at 时间戳）
	}

	// 3. 签发 JWT Token，包含用户 ID、OpenID 和当前家庭 ID（须为家庭成员）
	familyID := uint64(0)
	if user.CurrentFamilyID != nil {
		familyID = service.ResolveJWTFamilyID(h.db, user.ID, *user.CurrentFamilyID)
	}
	token, err := jwtPkg.Generate(config.AppConfig.JWT.Secret, config.AppConfig.JWT.ExpireHours, user.ID, user.OpenID, familyID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "生成token失败"})
		return
	}

	// 4. 返回 Token 和用户信息
	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "ok",
		"data": gin.H{
			"token":    token,
			"user_id":  user.ID,
			"openid":   user.OpenID,
			"nickname": user.Nickname,
			"avatar":   user.AvatarURL,
		},
	})
}

// GetProfile 获取当前登录用户信息接口。
//
// 路由：GET /api/users/me（需认证）
//
// 功能：
//   查询当前用户基本信息，同时判断在当前家庭中是否为厨师身份。
//
// 响应：
//   - 成功：{"code":0, "msg":"ok", "data":{"id":1,"openid":"...","nickname":"...","avatar_url":"...","current_family_id":1,"is_chef":true}}
//   - 失败：{"code":404, "msg":"用户不存在"}
func (h *AuthHandler) GetProfile(c *gin.Context) {
	// 从 JWT 上下文中获取当前用户 ID
	userID := middleware.GetUserID(c)

	// 1. 查询用户基本信息
	var user model.User
	if err := h.db.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "用户不存在"})
		return
	}

	// 2. 查询当前家庭的厨师身份标识
	var isChef bool
	if user.CurrentFamilyID != nil {
		var member model.FamilyMember
		// 在家庭成员表中查找该用户在当前家庭的记录
		if h.db.Where("family_id = ? AND user_id = ?", *user.CurrentFamilyID, userID).
			First(&member).Error == nil {
			isChef = member.IsChef // 读取 is_chef 字段
		}
	}

	// 3. 返回用户信息（含厨师身份）
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok", "data": gin.H{
		"id":                user.ID,
		"openid":            user.OpenID,
		"nickname":          user.Nickname,
		"avatar_url":        user.AvatarURL,
		"current_family_id": user.CurrentFamilyID,
		"is_chef":           isChef,
	}})
}

// updateProfileReq 更新用户信息请求体。
type updateProfileReq struct {
	Nickname        string  `json:"nickname"`          // 新昵称（非空才更新）
	AvatarURL       string  `json:"avatar_url"`        // 新头像 URL（非空才更新）
	CurrentFamilyID *uint64 `json:"current_family_id"` // 切换当前家庭（非空才更新）
}

// UpdateProfile 更新当前用户信息接口。
//
// 路由：PUT /api/users/me（需认证）
//
// 功能：
//   部分更新当前用户的昵称、头像或当前家庭。
//   仅更新非空字段（空值不覆盖旧值）。
//
// 请求 Body：
//   - nickname: string (可选)
//   - avatar_url: string (可选)
//   - current_family_id: uint64 (可选)
//
// 响应：
//   - 成功：{"code":0, "msg":"ok"}
//   - 失败：{"code":400, "msg":"参数错误"} / {"code":500, "msg":"更新失败"}
func (h *AuthHandler) UpdateProfile(c *gin.Context) {
	// 从 JWT 上下文中获取当前用户 ID
	userID := middleware.GetUserID(c)

	var req updateProfileReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "参数错误"})
		return
	}

	// 构建部分更新字段 map（仅回填非空值，避免误覆盖）
	updates := map[string]any{}
	if req.Nickname != "" {
		if respondSecCheck(c, service.DefaultSecCheck.CheckTexts(middleware.GetOpenID(c), service.SecCheckSceneProfile, req.Nickname)) {
			return
		}
		updates["nickname"] = req.Nickname
	}
	if req.AvatarURL != "" {
		updates["avatar_url"] = req.AvatarURL
	}
	if req.CurrentFamilyID != nil {
		if !service.IsFamilyMember(h.db, userID, *req.CurrentFamilyID) {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "不是该家庭的成员"})
			return
		}
		updates["current_family_id"] = *req.CurrentFamilyID
	}

	// 执行部分更新（GORM Updates + map 只更新非零值字段）
	if err := h.db.Model(&model.User{}).Where("id = ?", userID).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "更新失败"})
		return
	}

	resp := gin.H{"code": 0, "msg": "ok"}
	if req.CurrentFamilyID != nil {
		token, err := service.IssueUserJWT(h.db, userID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "生成token失败"})
			return
		}
		resp["data"] = gin.H{"token": token}
	}
	c.JSON(http.StatusOK, resp)
}
