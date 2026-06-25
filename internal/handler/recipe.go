// Package handler 提供 HTTP 请求处理器（Gin handlers）。
//
// 本文件 (recipe.go) 负责菜谱管理相关接口：
//   - 创建/更新/删除菜谱
//   - 查看菜谱详情
//   - 菜谱列表（分页、搜索、分类筛选）
//   - 标记菜谱已烹饪
package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"recipe-server/internal/middleware"
	"recipe-server/internal/model"
	"recipe-server/internal/service"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// RecipeHandler 菜谱管理处理器。
// 底层调用 RecipeService 处理业务逻辑。
type RecipeHandler struct {
	svc *service.RecipeService // 菜谱业务服务
	db  *gorm.DB               // 数据库连接（备用）
}

// NewRecipeHandler 创建菜谱处理器。
func NewRecipeHandler(db *gorm.DB) *RecipeHandler {
	return &RecipeHandler{svc: service.NewRecipeService(db), db: db}
}

// Create 创建菜谱接口。
//
// 路由：POST /api/recipes（需认证）
//
// 功能：
//   在当前家庭中创建一道新菜谱。
//   CreatorID 和 FamilyID 由服务端从 JWT 上下文自动注入。
//
// 请求 Body：
//   - name: string 菜名
//   - category: string 分类
//   - ingredients: string 食材
//   - steps: string 烹饪步骤
//   - image_url: string 图片 URL（可选）
//   - 其他模型字段...
//
// 响应：
//   - 成功：{"code":0, "msg":"ok", "data":{"id":1,"name":"...","family_id":1,...}}
//   - 失败：{"code":400, "msg":"参数错误"} / {"code":500, "msg":"创建失败: ..."}
func (h *RecipeHandler) Create(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "参数错误"})
		return
	}
	var r model.Recipe
	if err := json.Unmarshal(body, &r); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "参数错误"})
		return
	}
	if !bytes.Contains(body, []byte(`"is_public"`)) {
		r.IsPublic = false
	}

	// 2. 服务端注入创建者 ID 和家庭 ID（不信任客户端传入）
	r.CreatorID = middleware.GetUserID(c)
	r.FamilyID = middleware.GetFamilyID(c)
	if r.FamilyID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "请先加入家庭"})
		return
	}

	catSvc := service.NewCategoryService(h.db)
	category, err := catSvc.Ensure(r.FamilyID, r.Category)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "分类保存失败"})
		return
	}
	r.Category = category

	// 3. 调用 service 层创建菜谱
	if err := h.svc.Create(&r); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "创建失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok", "data": r})
}

// Update 更新菜谱接口。
//
// 路由：PUT /api/recipes/:id（需认证）
//
// 功能：
//   根据菜谱 ID 更新菜谱信息（支持部分更新）。
//   ID 同时来自路径参数和请求体，以路径参数为准。
//
// 路径参数：
//   - id: 菜谱 ID
//
// 请求 Body：
//   菜谱各字段（仅更新非零值）
//
// 响应：
//   - 成功：{"code":0, "msg":"ok"}
//   - 失败：{"code":400, "msg":"参数错误"} / {"code":500, "msg":"更新失败"}
func (h *RecipeHandler) Update(c *gin.Context) {
	// 从 URL 路径参数解析菜谱 ID
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)

	// 解析请求体
	var r model.Recipe
	if err := c.ShouldBindJSON(&r); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "参数错误"})
		return
	}

	// 以路径参数 ID 为准，覆盖请求体中的 ID（防止客户端篡改）
	r.ID = id
	familyID := middleware.GetFamilyID(c)
	if familyID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "请先加入家庭"})
		return
	}
	if strings.TrimSpace(r.Category) != "" {
		catSvc := service.NewCategoryService(h.db)
		if category, err := catSvc.Ensure(familyID, r.Category); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "分类保存失败"})
			return
		} else {
			r.Category = category
		}
	}
	if err := h.svc.Update(&r, familyID); err != nil {
		if errors.Is(err, service.ErrRecipeNotInFamily) {
			c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "菜谱不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "更新失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok"})
}

// Delete 删除菜谱接口（软删除）。
//
// 路由：DELETE /api/recipes/:id（需认证）
//
// 功能：
//   软删除指定菜谱。仅创建者可删除自己的菜谱。
//
// 路径参数：
//   - id: 菜谱 ID
//
// 响应：
//   - 成功：{"code":0, "msg":"ok"}
//   - 失败：{"code":500, "msg":"删除失败"}
func (h *RecipeHandler) Delete(c *gin.Context) {
	// 从 URL 路径参数解析菜谱 ID
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)

	familyID := middleware.GetFamilyID(c)
	if familyID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "请先加入家庭"})
		return
	}
	// 调用 service 层删除（校验是否为创建者本人且属于当前家庭）
	if err := h.svc.Delete(id, middleware.GetUserID(c), familyID); err != nil {
		if errors.Is(err, service.ErrRecipeNotInFamily) {
			c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "菜谱不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "删除失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok"})
}

// Get 获取菜谱详情接口。
//
// 路由：GET /api/recipes/:id（可选登录；本家庭或 is_public=1 可见）
//
// 功能：
//   根据菜谱 ID 查询菜谱详细信息。
//
// 路径参数：
//   - id: 菜谱 ID
//
// 响应：
//   - 成功：{"code":0, "msg":"ok", "data":{"id":1,"name":"...","ingredients":"...","steps":"...",...}}
//   - 失败：{"code":404, "msg":"菜谱不存在"}
func (h *RecipeHandler) Get(c *gin.Context) {
	// 从 URL 路径参数解析菜谱 ID
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)

	r, err := h.svc.GetByID(id, middleware.GetFamilyID(c))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "菜谱不存在"})
		return
	}
	userID := middleware.GetUserID(c)
	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "ok",
		"data": recipeWithFavorite{Recipe: *r, IsFavorited: isRecipeFavorited(h.db, userID, r.ID)},
	})
}

// List 获取菜谱列表接口（分页，支持搜索和分类筛选）。
//
// 路由：GET /api/recipes（可选登录；本家庭全部 + 其他家庭公开菜谱）
//
// 功能：
//   分页查询菜谱列表，支持关键字搜索和分类筛选。
//
// 查询参数：
//   - keyword: string (可选) 搜索关键字（模糊匹配菜名）
//   - category: string (可选) 分类筛选
//   - page: int (可选) 页码，默认 1
//   - page_size: int (可选) 每页条数，默认 20
//
// 响应：
//   - 成功：{"code":0, "data":{"list":[...],"total":100,"page":1,"page_size":20}}
//   - 失败：{"code":500, "msg":"查询失败"}
func (h *RecipeHandler) List(c *gin.Context) {
	page, pageSize := pageParams(c)

	// 调用 service 层分页查询（含关键字和分类筛选）
	recipes, total, err := h.svc.List(
		middleware.GetFamilyID(c),
		c.Query("keyword"),
		c.Query("category"),
		page, pageSize,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "查询失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"list":      recipesWithFavoriteFlags(h.db, middleware.GetUserID(c), recipes),
			"total":     total,
			"page":      page,
			"page_size": pageSize,
			"has_more":  int64(page*pageSize) < total,
		},
	})
}

// Cooked 标记菜谱已烹饪接口。
//
// 路由：POST /api/recipes/:id/cooked（需认证）
//
// 功能：
//   将指定菜谱的烹饪次数 +1（cook_count 递增）。
//
// 路径参数：
//   - id: 菜谱 ID
//
// 响应：
//   - 成功：{"code":0, "msg":"ok"}
//   - 失败：{"code":500, "msg":"操作失败"}
func (h *RecipeHandler) Cooked(c *gin.Context) {
	// 从 URL 路径参数解析菜谱 ID
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)

	familyID := middleware.GetFamilyID(c)
	if familyID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "请先加入家庭"})
		return
	}
	if err := h.svc.IncrementCookCount(id, familyID); err != nil {
		if errors.Is(err, service.ErrRecipeNotInFamily) {
			c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "菜谱不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "操作失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok"})
}
