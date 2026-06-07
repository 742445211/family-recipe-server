package handler

import (
	"net/http"
	"strconv"

	"recipe-server/internal/middleware"
	"recipe-server/internal/service"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// NotificationChannelHandler 通知通道配置接口。
type NotificationChannelHandler struct {
	svc *service.NotificationChannelService
}

func NewNotificationChannelHandler(db *gorm.DB) *NotificationChannelHandler {
	return &NotificationChannelHandler{svc: service.NewNotificationChannelService(db)}
}

type channelReq struct {
	Channel  string `json:"channel" binding:"required"`
	Enabled  *bool  `json:"enabled"`
	Endpoint string `json:"endpoint"`
	Secret   string `json:"secret"`
	Topic    string `json:"topic"`
}

// List GET /api/notification-channels
func (h *NotificationChannelHandler) List(c *gin.Context) {
	list, err := h.svc.ListByUser(middleware.GetUserID(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "查询失败"})
		return
	}
	if list == nil {
		list = []map[string]any{}
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": list})
}

// Create POST /api/notification-channels
func (h *NotificationChannelHandler) Create(c *gin.Context) {
	var req channelReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "参数错误"})
		return
	}
	ch, err := h.svc.Create(middleware.GetUserID(c), service.ChannelInput{
		Channel: req.Channel, Enabled: req.Enabled,
		Endpoint: req.Endpoint, Secret: req.Secret, Topic: req.Topic,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{
		"id": ch.ID, "channel": ch.Channel, "enabled": ch.Enabled,
	}})
}

// Update PUT /api/notification-channels/:id
func (h *NotificationChannelHandler) Update(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req channelReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "参数错误"})
		return
	}
	if err := h.svc.Update(middleware.GetUserID(c), id, service.ChannelInput{
		Enabled: req.Enabled, Endpoint: req.Endpoint, Secret: req.Secret, Topic: req.Topic,
	}); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok"})
}

// Delete DELETE /api/notification-channels/:id
func (h *NotificationChannelHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.svc.Delete(middleware.GetUserID(c), id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok"})
}
