package service

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"recipe-server/config"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var imageWorkerUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// ImageWorkerHub 管理树莓派图片处理网关 WebSocket 连接。
type ImageWorkerHub struct {
	mu       sync.RWMutex
	conn     *websocket.Conn
	workerID string
	onResult func([]byte)
}

func NewImageWorkerHub(onResult func([]byte)) *ImageWorkerHub {
	return &ImageWorkerHub{onResult: onResult}
}

func (h *ImageWorkerHub) IsConnected() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.conn != nil
}

func (h *ImageWorkerHub) SendTask(payload map[string]any) bool {
	h.mu.RLock()
	conn := h.conn
	h.mu.RUnlock()
	if conn == nil {
		return false
	}
	data, _ := json.Marshal(payload)
	log.Printf("[ImageWorker] send task: %s", string(data))
	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		log.Printf("[ImageWorker] send task failed: %v", err)
		return false
	}
	return true
}

func (h *ImageWorkerHub) HandleWebSocket(c *gin.Context) {
	if !requireSecureConnection(c) {
		c.JSON(http.StatusForbidden, gin.H{"code": 403, "msg": "requires HTTPS/WSS"})
		return
	}
	token := c.Query("token")
	if token == "" || token != config.AppConfig.ImageWorker.Token {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": "invalid gateway token"})
		return
	}
	workerID := c.Query("worker_id")
	if workerID == "" {
		workerID = "unknown"
	}

	conn, err := imageWorkerUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("[ImageWorker] upgrade failed: %v", err)
		return
	}

	h.mu.Lock()
	if h.conn != nil {
		_ = h.conn.Close()
	}
	h.conn = conn
	h.workerID = workerID
	h.mu.Unlock()

	log.Printf("[ImageWorker] gateway connected worker_id=%s", workerID)
	_ = conn.WriteJSON(map[string]any{
		"type":      "registered",
		"worker_id": workerID,
	})

	readSec := config.AppConfig.ImageWorker.ReadTimeoutSec
	pingSec := config.AppConfig.ImageWorker.PingIntervalSec
	_ = conn.SetReadDeadline(time.Now().Add(time.Duration(readSec) * time.Second))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(time.Duration(readSec) * time.Second))
	})

	go func() {
		ticker := time.NewTicker(time.Duration(pingSec) * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			h.mu.RLock()
			c := h.conn
			h.mu.RUnlock()
			if c == nil {
				return
			}
			if err := c.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}()

	defer func() {
		h.mu.Lock()
		if h.conn == conn {
			h.conn = nil
			h.workerID = ""
		}
		h.mu.Unlock()
		_ = conn.Close()
		log.Printf("[ImageWorker] gateway disconnected worker_id=%s", workerID)
	}()

	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			break
		}
		var base struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(data, &base); err != nil {
			continue
		}
		switch base.Type {
		case "ping":
			_ = conn.WriteJSON(map[string]string{"type": "pong"})
		case "task_result", "result":
			log.Printf("[ImageWorker] recv %s: %s", base.Type, truncateLog(string(data), 512))
			if h.onResult != nil {
				go h.onResult(data)
			}
		default:
			log.Printf("[ImageWorker] recv unknown type=%s: %s", base.Type, truncateLog(string(data), 256))
		}
	}
}

func requireSecureConnection(c *gin.Context) bool {
	if c.Request.TLS != nil {
		return true
	}
	proto := strings.ToLower(c.GetHeader("X-Forwarded-Proto"))
	return proto == "https" || proto == "wss"
}

func truncateLog(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
