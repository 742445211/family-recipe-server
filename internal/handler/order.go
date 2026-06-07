// Package handler 提供 HTTP 请求处理器（Gin handlers）。
//
// 本文件 (order.go) 负责每日点菜相关接口：
//   - 点一道菜
//   - 查看某天某餐次的点菜列表
//   - 取消点菜（软删除）
//   点菜成功后自动异步通知家庭厨师。
package handler

import (
	"net/http"
	"strconv"
	"time"

	"recipe-server/internal/middleware"
	"recipe-server/internal/model"
	"recipe-server/internal/service"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// today 返回今天的日期字符串（格式：YYYY-MM-DD）。
// 用于点菜时默认日期和查询时默认日期。
func today() string {
	return time.Now().Format("2006-01-02")
}

// OrderHandler 每日点菜处理器。
// 底层调用 OrderService 处理业务逻辑。
type OrderHandler struct {
	svc         *service.OrderService
	notifySvc   *service.NotificationService
}

// NewOrderHandler 创建点菜处理器。
func NewOrderHandler(db *gorm.DB, hub *service.WebSocketHub) *OrderHandler {
	return &OrderHandler{
		svc:       service.NewOrderService(db),
		notifySvc: service.NewNotificationService(db, hub),
	}
}

// addOrderReq 点菜请求体。
type addOrderReq struct {
	RecipeID uint64 `json:"recipe_id" binding:"required"` // 菜谱 ID（必填）
	Date     string `json:"date"`                         // 日期 YYYY-MM-DD（默认今天）
	MealType string `json:"meal_type"`                    // 餐次：breakfast/lunch/dinner（默认 dinner）
	Quantity int    `json:"quantity"`                     // 份数（默认 1）
	Note     string `json:"note"`                         // 备注说明
}

// Add 点一道菜接口。
//
// 路由：POST /api/orders（需认证）
//
// 功能：
//   在当前家庭中点一道菜，同餐次内同一菜谱不可重复点。
//   成功后异步推送订阅消息通知家庭厨师。
//
// 请求 Body：
//   - recipe_id: uint64 (必填) 菜谱 ID
//   - date: string (可选) 日期，格式 YYYY-MM-DD，默认今天
//   - meal_type: string (可选) 餐次 breakfast/lunch/dinner，默认 dinner
//   - quantity: int (可选) 份数，默认 1
//   - note: string (可选) 备注
//
// 响应：
//   - 成功：{"code":0, "msg":"ok", "data":{"id":1,"family_id":1,"recipe_id":5,"date":"2026-05-03",...}}
//   - 失败：{"code":400, "msg":"参数错误: recipe_id必填"/"该餐次已点过这道菜"} / {"code":500, "msg":"点菜失败"}
func (h *OrderHandler) Add(c *gin.Context) {
	var req addOrderReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "参数错误: recipe_id必填"})
		return
	}

	// 默认值处理
	if req.Date == "" {
		req.Date = today() // 未传日期则默认今天
	}
	if req.MealType == "" {
		req.MealType = "dinner" // 未传餐次则默认晚餐
	}
	if req.Quantity <= 0 {
		req.Quantity = 1 // 份数非法则默认为 1
	}

	// 调用 service 层创建点菜记录
	order, err := h.svc.Add(
		middleware.GetFamilyID(c),
		req.RecipeID,
		req.MealType,
		middleware.GetUserID(c),
		req.Date, req.Note, req.Quantity,
	)
	if err != nil {
		// 区分业务错误（如重复点菜）和系统错误
		if err.Error() == "该餐次已点过这道菜" {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "点菜失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok", "data": order})

	familyID := middleware.GetFamilyID(c)
	orderDate := req.Date
	orderID := order.ID
	db := h.svc.DB()

	h.notifySvc.NotifyOrderCreatedAsync(orderID)
	go service.UpdateChefServiceCard(db, familyID)
	go service.UpdateDynamicMessages(db, familyID, orderDate)
}

// List 获取某天某餐次的点菜列表接口。
//
// 路由：GET /api/orders（需认证）
//
// 功能：
//   查询当前家庭在指定日期和餐次的所有点菜记录。
//
// 查询参数：
//   - date: string (可选) 日期 YYYY-MM-DD，默认今天
//   - meal_type: string (可选) 餐次，空值表示查询该天所有餐次
//
// 响应：
//   - 成功：{"code":0, "data":[{"id":1,"recipe_id":5,"recipe":{...},"quantity":2,...}]}
//   - 失败：{"code":500, "msg":"查询失败"}
func (h *OrderHandler) List(c *gin.Context) {
	date := c.DefaultQuery("date", today())
	mealType := c.DefaultQuery("meal_type", "")

	// 调用 service 层查询
	orders, err := h.svc.GetByDateAndMeal(middleware.GetFamilyID(c), date, mealType)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "查询失败"})
		return
	}

	// 确保返回空数组而非 null（前端友好）
	if orders == nil {
		orders = []model.DailyOrder{}
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": orders})
}

// Remove 取消点菜接口（软删除）。
//
// 路由：DELETE /api/orders/:id（需认证）
//
// 功能：
//   软删除一条点菜记录（设置 deleted_at 时间戳）。
//   仅点菜人本人可取消自己的点菜。
//
// 路径参数：
//   - id: 点菜记录 ID
//
// 响应：
//   - 成功：{"code":0, "msg":"ok"}
//   - 失败：{"code":400, "msg":"..."}
func (h *OrderHandler) Remove(c *gin.Context) {
	// 从 URL 路径参数解析点菜记录 ID
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)

	// 调用 service 层软删除（校验是否为点菜人本人）
	if err := h.svc.Remove(id, middleware.GetUserID(c)); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok"})
}

// Share 创建动态消息 activity_id，供前端分享菜单卡片到群聊。
//
// 路由：POST /api/orders/share（需认证）
//
// 功能：
//   调用微信 API 创建 activity_id，存入内存映射表，
//   后续点菜时自动更新该卡片内容。
//
// 响应：
//   - 成功：{"code":0, "data":{"activity_id":"xxx"}}
//   - 失败：{"code":500, "msg":"创建失败"}
func (h *OrderHandler) Share(c *gin.Context) {
	activityID, err := service.CreateActivityID()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "创建分享失败: " + err.Error()})
		return
	}

	// 记录 activity_id 对应关系，后续点菜时更新卡片
	service.StoreActivity(activityID, middleware.GetFamilyID(c), today())

	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{"activity_id": activityID}})
}
