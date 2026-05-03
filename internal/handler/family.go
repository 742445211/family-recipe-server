// Package handler 提供 HTTP 请求处理器（Gin handlers）。
//
// 本文件 (family.go) 负责家庭管理相关接口：
//   - 创建家庭（生成邀请码）
//   - 通过邀请码加入家庭
//   - 查看家庭列表
//   - 查看家庭成员
//   - 切换厨师身份
package handler

import (
	"math/rand"
	"net/http"
	"strconv"

	"recipe-server/internal/middleware"
	"recipe-server/internal/model"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// FamilyHandler 处理家庭相关的 HTTP 请求（创建、加入、列表、成员、厨师切换）。
type FamilyHandler struct {
	db *gorm.DB // 数据库连接
}

// NewFamilyHandler 创建家庭处理器。
func NewFamilyHandler(db *gorm.DB) *FamilyHandler {
	return &FamilyHandler{db: db}
}

// Create 创建新家庭接口。
//
// 路由：POST /api/families（需认证）
//
// 功能：
//   创建一个新家庭，自动生成 6 位字母数字邀请码。
//   创建者自动成为 family owner，并设为当前家庭。
//
// 请求 Body：
//   - name: string (必填) 家庭名称
//
// 响应：
//   - 成功：{"code":0, "msg":"ok", "data":{"id":1,"name":"...","invite_code":"ABC123",...}}
//   - 失败：{"code":400, "msg":"参数错误"/"家庭名称不能为空"} / {"code":500, "msg":"创建失败"}
func (h *FamilyHandler) Create(c *gin.Context) {
	// 1. 解析请求体
	var f model.Family
	if err := c.ShouldBindJSON(&f); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "参数错误"})
		return
	}
	if f.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "家庭名称不能为空"})
		return
	}

	// 2. 从 JWT 上下文中获取当前用户 ID
	userID := middleware.GetUserID(c)

	// 3. 生成 6 位邀请码（大写字母+数字，排除易混淆的 0/O/1/I 等字符）
	f.InviteCode = generateCode(6)

	// 4. 创建家庭记录
	if err := h.db.Create(&f).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "创建失败"})
		return
	}

	// 5. 创建者设为 owner 角色并加入家庭（写入 family_members 表）
	member := model.FamilyMember{
		FamilyID: f.ID,
		UserID:   userID,
		Role:     "owner",
	}
	h.db.Create(&member)

	// 6. 自动将新家庭设为当前用户的 current_family_id
	h.db.Model(&model.User{}).Where("id = ?", userID).Update("current_family_id", f.ID)

	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok", "data": f})
}

// joinReq 加入家庭请求体。
type joinReq struct {
	InviteCode string `json:"invite_code" binding:"required"` // 6位邀请码
}

// Join 通过邀请码加入家庭接口。
//
// 路由：POST /api/families/join（需认证）
//
// 功能：
//   根据邀请码查找家庭，将当前用户添加为 member（幂等：已加入则忽略）。
//   加入成功后自动设为当前家庭。
//
// 请求 Body：
//   - invite_code: string (必填) 6位邀请码
//
// 响应：
//   - 成功：{"code":0, "msg":"ok", "data":{"id":1,"name":"...","invite_code":"ABC123",...}}
//   - 失败：{"code":400, "msg":"邀请码必填"} / {"code":404, "msg":"邀请码无效"} / {"code":500, "msg":"加入失败"}
func (h *FamilyHandler) Join(c *gin.Context) {
	var req joinReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "邀请码必填"})
		return
	}

	// 1. 根据邀请码查找对应家庭
	var family model.Family
	if err := h.db.Where("invite_code = ?", req.InviteCode).First(&family).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "邀请码无效"})
		return
	}

	// 2. 获取当前用户 ID
	userID := middleware.GetUserID(c)

	// 3. 创建家庭成员关联（默认角色为 member）
	member := model.FamilyMember{
		FamilyID: family.ID,
		UserID:   userID,
		Role:     "member",
	}

	// 4. FirstOrCreate：如果已加入则忽略，否则插入新记录（幂等操作）
	if err := h.db.Where(model.FamilyMember{FamilyID: family.ID, UserID: userID}).
		FirstOrCreate(&member).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "加入失败"})
		return
	}

	// 5. 设为当前家庭
	h.db.Model(&model.User{}).Where("id = ?", userID).Update("current_family_id", family.ID)

	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok", "data": family})
}

// List 获取当前用户加入的所有家庭列表接口。
//
// 路由：GET /api/families（需认证）
//
// 功能：
//   查询当前用户所在的所有家庭（通过 family_members 关联表）。
//
// 响应：
//   - 成功：{"code":0, "data":[{"id":1,"name":"...","invite_code":"ABC123",...}]}
func (h *FamilyHandler) List(c *gin.Context) {
	// 从 JWT 上下文中获取当前用户 ID
	userID := middleware.GetUserID(c)

	// 查询用户所有的家庭成员记录，同时预加载关联的家庭信息
	var members []model.FamilyMember
	h.db.Where("user_id = ?", userID).Preload("Family").Find(&members)

	// 从关联关系中提取家庭列表（排除 nil）
	families := make([]model.Family, 0, len(members))
	for _, m := range members {
		if m.Family != nil {
			families = append(families, *m.Family)
		}
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": families})
}

// Members 获取指定家庭的成员列表接口。
//
// 路由：GET /api/families/:id/members（需认证）
//
// 功能：
//   查询指定家庭的所有成员，含每个成员的用户信息（昵称、头像等）。
//
// 路径参数：
//   - id: 家庭 ID
//
// 响应：
//   - 成功：{"code":0, "data":[{"family_id":1,"user_id":2,"role":"owner","is_chef":false,"user":{...}}]}
func (h *FamilyHandler) Members(c *gin.Context) {
	// 从 URL 路径参数中解析家庭 ID
	familyID, _ := strconv.ParseUint(c.Param("id"), 10, 64)

	// 查询该家庭的所有成员，预加载 User 关联信息
	var members []model.FamilyMember
	h.db.Where("family_id = ?", familyID).Preload("User").Find(&members)

	c.JSON(http.StatusOK, gin.H{"code": 0, "data": members})
}

// ToggleChef 切换当前用户在当前家庭的厨师身份接口。
//
// 路由：POST /api/families/chef（需认证）
//
// 功能：
//   将当前用户的厨师身份取反：是厨师→取消厨师，不是厨师→设为厨师。
//
// 响应：
//   - 成功：{"code":0, "msg":"ok", "data":{"is_chef":true}}
//   - 失败：{"code":400, "msg":"请先选择家庭"} / {"code":404, "msg":"你不是该家庭的成员"} / {"code":500, "msg":"操作失败"}
func (h *FamilyHandler) ToggleChef(c *gin.Context) {
	// 从 JWT 上下文中获取用户 ID 和家庭 ID
	userID := middleware.GetUserID(c)
	familyID := middleware.GetFamilyID(c)

	// 校验用户是否已选择家庭（family_id 为 0 表示未选择）
	if familyID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "请先选择家庭"})
		return
	}

	// 校验是否为该家庭成员
	var member model.FamilyMember
	if err := h.db.Where("family_id = ? AND user_id = ?", familyID, userID).First(&member).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "你不是该家庭的成员"})
		return
	}

	// 切换厨师身份：取反
	newVal := !member.IsChef
	tx := h.db.Model(&member).UpdateColumn("is_chef", newVal)
	if tx.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "操作失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok", "data": gin.H{"is_chef": newVal}})
}

// codeChars 邀请码可用字符集。
// 排除易混淆字符：0（数字零）、O（大写字母O）、1（数字一）、I（大写字母I）等。
const codeChars = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

// generateCode 生成指定长度的随机邀请码。
// 从 codeChars 字符集中随机选择 n 个字符组成邀请码。
func generateCode(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = codeChars[rand.Intn(len(codeChars))]
	}
	return string(b)
}
