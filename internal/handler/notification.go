package handler

import (
	"net/http"
	"strconv"

	"recipe-server/internal/middleware"
	"recipe-server/internal/model"
	"recipe-server/internal/service"
	"recipe-server/pkg/dateutil"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// NotificationHandler 通知接口。
type NotificationHandler struct {
	svc *service.NotificationService
}

func NewNotificationHandler(db *gorm.DB, hub *service.WebSocketHub) *NotificationHandler {
	return &NotificationHandler{svc: service.NewNotificationService(db, hub)}
}

// ListUnread GET /api/notifications/unread
func (h *NotificationHandler) ListUnread(c *gin.Context) {
	list, err := h.svc.ListUnread(middleware.GetUserID(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "查询失败"})
		return
	}
	if list == nil {
		list = []model.Notification{}
	}
	// 附加订单日期/餐次，方便前端跳转
	type item struct {
		model.Notification
		OrderDate string `json:"order_date,omitempty"`
		MealType  string `json:"meal_type,omitempty"`
	}
	out := make([]item, 0, len(list))
	for _, n := range list {
		it := item{Notification: n}
		var order model.DailyOrder
		if h.svc.DB().Select("date", "meal_type").First(&order, n.OrderID).Error == nil {
			it.OrderDate = dateutil.FormatYMD(order.Date)
			it.MealType = order.MealType
		}
		out = append(out, it)
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": out})
}

// MarkRead POST /api/notifications/:id/read
func (h *NotificationHandler) MarkRead(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.svc.MarkRead(middleware.GetUserID(c), id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "通知不存在或已读"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok"})
}
