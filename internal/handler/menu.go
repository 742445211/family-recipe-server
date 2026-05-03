// Package handler 提供 HTTP 请求处理器（Gin handlers）。
//
// 本文件 (menu.go) 负责菜单管理相关接口：
//   - 创建菜单
//   - 查看菜单详情/列表
//   - 向菜单添加菜品项
//   - 从菜单移除菜品项
//   - 确认菜单
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

// MenuHandler 菜单管理处理器。
// 提供菜单的 CRUD 及菜品项管理功能，底层调用 MenuService 处理业务逻辑。
type MenuHandler struct {
	svc *service.MenuService // 菜单业务服务
	db  *gorm.DB             // 数据库连接（备用）
}

// NewMenuHandler 创建菜单处理器。
func NewMenuHandler(db *gorm.DB) *MenuHandler {
	return &MenuHandler{svc: service.NewMenuService(db), db: db}
}

// Create 创建菜单接口。
//
// 路由：POST /api/menus（推测，需认证）
//
// 功能：
//   为当前家庭创建一个新菜单（如"本周菜单"）。
//   FamilyID 和 CreatorID 由服务端从 JWT 上下文自动注入。
//
// 请求 Body：
//   - name: string 菜单名称
//   - description: string 菜单描述（可选）
//
// 响应：
//   - 成功：{"code":0, "msg":"ok", "data":{"id":1,"family_id":1,"name":"...",...}}
//   - 失败：{"code":400, "msg":"参数错误"} / {"code":500, "msg":"创建失败"}
func (h *MenuHandler) Create(c *gin.Context) {
	// 1. 解析请求体到 Menu 结构体
	var m model.Menu
	if err := c.ShouldBindJSON(&m); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "参数错误"})
		return
	}

	// 2. 服务端注入家庭 ID 和创建者 ID（不信任客户端传入）
	m.FamilyID = middleware.GetFamilyID(c)
	m.CreatorID = middleware.GetUserID(c)

	// 3. 调用 service 层创建菜单
	if err := h.svc.Create(&m); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "创建失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok", "data": m})
}

// Get 获取菜单详情接口。
//
// 路由：GET /api/menus/:id（推测，需认证）
//
// 功能：
//   根据菜单 ID 查询菜单详细信息（含菜单内的菜品项列表）。
//
// 路径参数：
//   - id: 菜单 ID
//
// 响应：
//   - 成功：{"code":0, "msg":"ok", "data":{"id":1,"name":"...","items":[...]}}
//   - 失败：{"code":404, "msg":"菜单不存在"}
func (h *MenuHandler) Get(c *gin.Context) {
	// 从 URL 路径参数解析菜单 ID
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)

	// 调用 service 层获取菜单详情
	m, err := h.svc.GetByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "菜单不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok", "data": m})
}

// List 获取菜单列表接口（分页）。
//
// 路由：GET /api/menus（推测，需认证）
//
// 功能：
//   分页查询当前家庭的菜单列表。
//
// 查询参数：
//   - page: int 页码（默认 1）
//   - page_size: int 每页条数（默认 20）
//
// 响应：
//   - 成功：{"code":0, "data":{"list":[...],"total":10,"page":1,"page_size":20}}
//   - 失败：{"code":500, "msg":"查询失败"}
func (h *MenuHandler) List(c *gin.Context) {
	// 从 JWT 上下文中获取当前家庭 ID
	familyID := middleware.GetFamilyID(c)

	// 解析分页参数（带默认值）
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	// 调用 service 层分页查询
	menus, total, err := h.svc.List(familyID, page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "查询失败"})
		return
	}

	// 返回分页结果
	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"list":      menus,
			"total":     total,
			"page":      page,
			"page_size": pageSize,
		},
	})
}

// addItemReq 向菜单添加菜品项的请求体。
type addItemReq struct {
	RecipeID uint64 `json:"recipe_id" binding:"required"` // 菜谱 ID（必填）
	Quantity int    `json:"quantity"`                     // 份数（默认 1）
	Note     string `json:"note"`                         // 备注说明
}

// AddItem 向菜单添加菜品项接口。
//
// 路由：POST /api/menus/:id/items（推测，需认证）
//
// 功能：
//   向指定菜单中添加一道菜品，记录菜品、份数和备注。
//   份数为 0 或负数时自动修正为 1。
//
// 路径参数：
//   - id: 菜单 ID
//
// 请求 Body：
//   - recipe_id: uint64 (必填) 菜谱 ID
//   - quantity: int (可选) 份数，默认 1
//   - note: string (可选) 备注
//
// 响应：
//   - 成功：{"code":0, "msg":"ok", "data":{"id":1,"menu_id":1,"recipe_id":5,"quantity":1,...}}
//   - 失败：{"code":400, "msg":"参数错误: recipe_id必填"} / {"code":400, "msg":"..."}
func (h *MenuHandler) AddItem(c *gin.Context) {
	// 从 URL 路径参数解析菜单 ID
	menuID, _ := strconv.ParseUint(c.Param("id"), 10, 64)

	var req addItemReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "参数错误: recipe_id必填"})
		return
	}

	// 份数校验：≤0 时默认设为 1
	if req.Quantity <= 0 {
		req.Quantity = 1
	}

	// 调用 service 层添加菜品项
	item, err := h.svc.AddItem(menuID, req.RecipeID, middleware.GetUserID(c), req.Quantity, req.Note)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok", "data": item})
}

// RemoveItem 从菜单移除菜品项接口。
//
// 路由：DELETE /api/menus/:id/items/:item_id（推测，需认证）
//
// 功能：
//   从指定菜单中移除一个菜品项。
//
// 路径参数：
//   - id: 菜单 ID
//   - item_id: 菜品项 ID
//
// 响应：
//   - 成功：{"code":0, "msg":"ok", "data":{"menu_id":1}}
//   - 失败：{"code":400, "msg":"..."}
func (h *MenuHandler) RemoveItem(c *gin.Context) {
	// 从 URL 路径参数解析菜单 ID 和菜品项 ID
	menuID, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	itemID, _ := strconv.ParseUint(c.Param("item_id"), 10, 64)

	// 调用 service 层移除菜品项（带用户身份校验）
	if err := h.svc.RemoveItem(itemID, middleware.GetUserID(c)); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok", "data": gin.H{"menu_id": menuID}})
}

// Confirm 确认菜单接口。
//
// 路由：POST /api/menus/:id/confirm（推测，需认证）
//
// 功能：
//   将菜单状态变更为"已确认"（如从草稿变为已确认状态）。
//
// 路径参数：
//   - id: 菜单 ID
//
// 响应：
//   - 成功：{"code":0, "msg":"ok"}
//   - 失败：{"code":500, "msg":"确认失败"}
func (h *MenuHandler) Confirm(c *gin.Context) {
	// 从 URL 路径参数解析菜单 ID
	menuID, _ := strconv.ParseUint(c.Param("id"), 10, 64)

	// 调用 service 层确认菜单
	if err := h.svc.ConfirmMenu(menuID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "确认失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok"})
}
