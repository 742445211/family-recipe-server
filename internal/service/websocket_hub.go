// Package service - 用户 WebSocket 连接 Hub。
//
// 管理小程序端的在线连接（JWT query token 鉴权），支持同一用户多端并存；
// 用于通知实时推送，连接建立时可触发 onConnect 回调补推离线消息。
package service

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"recipe-server/config"
	"recipe-server/pkg/jwt"

	"github.com/gin-gonic/gin"
	jwtlib "github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true
		}
		host := r.Host
		if strings.Contains(origin, host) {
			return true
		}
		if config.AppConfig != nil && config.AppConfig.Server.AllowedOrigins != nil {
			for _, allowed := range config.AppConfig.Server.AllowedOrigins {
				if origin == allowed {
					return true
				}
			}
		}
		return false
	},
}

// WebSocketHub 管理在线 WebSocket 连接。
type WebSocketHub struct {
	mu          sync.RWMutex
	connections map[uint64]map[*websocket.Conn]struct{}
	onConnect   func(userID uint64)
}

// NewWebSocketHub 创建 Hub。
func NewWebSocketHub() *WebSocketHub {
	return &WebSocketHub{connections: make(map[uint64]map[*websocket.Conn]struct{})}
}

// SetOnConnect 设置用户连接成功后的回调（用于补推离线期间的通知等）。
func (h *WebSocketHub) SetOnConnect(fn func(userID uint64)) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.onConnect = fn
}

func (h *WebSocketHub) triggerOnConnect(userID uint64) {
	h.mu.RLock()
	fn := h.onConnect
	h.mu.RUnlock()
	if fn != nil {
		go fn(userID)
	}
}

func (h *WebSocketHub) register(userID uint64, conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.connections[userID] == nil {
		h.connections[userID] = make(map[*websocket.Conn]struct{})
	}
	h.connections[userID][conn] = struct{}{}
}

func (h *WebSocketHub) unregister(userID uint64, conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if conns, ok := h.connections[userID]; ok {
		delete(conns, conn)
		if len(conns) == 0 {
			delete(h.connections, userID)
		}
	}
}

// IsOnline 用户是否在线。
func (h *WebSocketHub) IsOnline(userID uint64) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.connections[userID]) > 0
}

// PushToUser 向用户所有连接推送消息。
func (h *WebSocketHub) PushToUser(userID uint64, payload map[string]any) bool {
	h.mu.RLock()
	conns := h.connections[userID]
	copies := make([]*websocket.Conn, 0, len(conns))
	for c := range conns {
		copies = append(copies, c)
	}
	h.mu.RUnlock()
	if len(copies) == 0 {
		return false
	}
	data, _ := json.Marshal(payload)
	sent := false
	for _, conn := range copies {
		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			log.Printf("[WebSocket] 推送失败 userID=%d: %v", userID, err)
			h.unregister(userID, conn)
			_ = conn.Close()
			continue
		}
		sent = true
	}
	return sent
}

// HandleWebSocket Gin WebSocket 处理器。
func (h *WebSocketHub) HandleWebSocket(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		auth := c.GetHeader("Authorization")
		if strings.HasPrefix(auth, "Bearer ") {
			token = strings.TrimSpace(strings.TrimPrefix(auth, "Bearer "))
		}
	}
	if token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": "未登录"})
		return
	}
	claims, err := jwt.Parse(config.AppConfig.JWT.Secret, token)
	if err != nil {
		msg := "Token无效"
		if errors.Is(err, jwtlib.ErrTokenExpired) {
			msg = "Token已过期，请重新登录"
		}
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": msg})
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("[WebSocket] 升级失败: %v", err)
		return
	}

	userID := claims.UserID
	h.register(userID, conn)
	log.Printf("[WebSocket] 用户 %d 已连接", userID)
	h.triggerOnConnect(userID)

	pingSec := config.AppConfig.Notification.WebSocket.PingIntervalSec
	readSec := config.AppConfig.Notification.WebSocket.ReadTimeoutSec
	_ = conn.SetReadDeadline(time.Now().Add(time.Duration(readSec) * time.Second))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(time.Duration(readSec) * time.Second))
	})

	go func() {
		ticker := time.NewTicker(time.Duration(pingSec) * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				break
			}
		}
	}()

	defer func() {
		h.unregister(userID, conn)
		_ = conn.Close()
		log.Printf("[WebSocket] 用户 %d 已断开", userID)
	}()

	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			break
		}
	}
}
