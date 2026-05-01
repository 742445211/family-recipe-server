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

type FamilyHandler struct {
	db *gorm.DB
}

func NewFamilyHandler(db *gorm.DB) *FamilyHandler {
	return &FamilyHandler{db: db}
}

func (h *FamilyHandler) Create(c *gin.Context) {
	var f model.Family
	if err := c.ShouldBindJSON(&f); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "参数错误"})
		return
	}
	if f.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "家庭名称不能为空"})
		return
	}

	userID := middleware.GetUserID(c)

	// 生成6位邀请码
	f.InviteCode = generateCode(6)
	if err := h.db.Create(&f).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "创建失败"})
		return
	}

	// 创建者为owner
	member := model.FamilyMember{
		FamilyID: f.ID,
		UserID:   userID,
		Role:     "owner",
	}
	h.db.Create(&member)

	// 设为当前家庭
	h.db.Model(&model.User{}).Where("id = ?", userID).Update("current_family_id", f.ID)

	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok", "data": f})
}

type joinReq struct {
	InviteCode string `json:"invite_code" binding:"required"`
}

func (h *FamilyHandler) Join(c *gin.Context) {
	var req joinReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "邀请码必填"})
		return
	}

	var family model.Family
	if err := h.db.Where("invite_code = ?", req.InviteCode).First(&family).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "邀请码无效"})
		return
	}

	userID := middleware.GetUserID(c)
	member := model.FamilyMember{
		FamilyID: family.ID,
		UserID:   userID,
		Role:     "member",
	}
	if err := h.db.Where(model.FamilyMember{FamilyID: family.ID, UserID: userID}).
		FirstOrCreate(&member).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "加入失败"})
		return
	}

	// 设为当前家庭
	h.db.Model(&model.User{}).Where("id = ?", userID).Update("current_family_id", family.ID)

	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok", "data": family})
}

func (h *FamilyHandler) List(c *gin.Context) {
	userID := middleware.GetUserID(c)
	var members []model.FamilyMember
	h.db.Where("user_id = ?", userID).Preload("Family").Find(&members)

	families := make([]model.Family, 0, len(members))
	for _, m := range members {
		if m.Family != nil {
			families = append(families, *m.Family)
		}
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": families})
}

func (h *FamilyHandler) Members(c *gin.Context) {
	familyID, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var members []model.FamilyMember
	h.db.Where("family_id = ?", familyID).Preload("User").Find(&members)
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": members})
}

// ToggleChef 切换当前用户的厨师身份
func (h *FamilyHandler) ToggleChef(c *gin.Context) {
	userID := middleware.GetUserID(c)
	familyID := middleware.GetFamilyID(c)
	if familyID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "请先选择家庭"})
		return
	}

	var member model.FamilyMember
	if err := h.db.Where("family_id = ? AND user_id = ?", familyID, userID).First(&member).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "你不是该家庭的成员"})
		return
	}

	// 切换厨师身份
	newVal := !member.IsChef
	tx := h.db.Model(&member).UpdateColumn("is_chef", newVal)
	if tx.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "操作失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok", "data": gin.H{"is_chef": newVal}})
}

const codeChars = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

func generateCode(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = codeChars[rand.Intn(len(codeChars))]
	}
	return string(b)
}
