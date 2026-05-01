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

func today() string {
	return time.Now().Format("2006-01-02")
}

type OrderHandler struct {
	svc *service.OrderService
}

func NewOrderHandler(db *gorm.DB) *OrderHandler {
	return &OrderHandler{svc: service.NewOrderService(db)}
}

type addOrderReq struct {
	RecipeID uint64 `json:"recipe_id" binding:"required"`
	Date     string `json:"date"`     // 默认今天
	Quantity int    `json:"quantity"`
	Note     string `json:"note"`
}

// Add — 点一道菜
func (h *OrderHandler) Add(c *gin.Context) {
	var req addOrderReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "参数错误: recipe_id必填"})
		return
	}
	if req.Date == "" {
		req.Date = today()
	}
	if req.Quantity <= 0 {
		req.Quantity = 1
	}

	order, err := h.svc.Add(
		middleware.GetFamilyID(c),
		req.RecipeID,
		middleware.GetUserID(c),
		req.Date, req.Note, req.Quantity,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "点菜失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok", "data": order})
}

// List — 获取某天的点菜列表（默认今天）
func (h *OrderHandler) List(c *gin.Context) {
	date := c.DefaultQuery("date", today())
	orders, err := h.svc.GetByDate(middleware.GetFamilyID(c), date)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "查询失败"})
		return
	}
	if orders == nil {
		orders = []model.DailyOrder{}
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": orders})
}

// Remove — 取消点菜
func (h *OrderHandler) Remove(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.svc.Remove(id, middleware.GetUserID(c)); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok"})
}
