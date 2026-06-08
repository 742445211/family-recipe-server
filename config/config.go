// Package config 提供应用配置的加载与管理功能。
// 配置结构体映射 config.yaml 文件，包含服务器、数据库、JWT、
// 微信小程序、阿里云 OSS 以及 AI 服务的配置项。
package config

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config 全局应用配置，对应 config.yaml 顶层结构。
type Config struct {
	Server       ServerConfig       `yaml:"server"`       // 服务器配置
	MySQL        MySQLConfig        `yaml:"mysql"`        // MySQL 数据库配置
	Redis        RedisConfig        `yaml:"redis"`        // Redis 缓存配置
	JWT          JWTConfig          `yaml:"jwt"`          // JWT 认证配置
	WeChat       WeChatConfig       `yaml:"wechat"`       // 微信小程序配置
	OSS          OSSConfig          `yaml:"oss"`          // 阿里云 OSS 配置
	AI           AIConfig           `yaml:"ai"`           // AI 服务配置
	Weather      WeatherConfig      `yaml:"weather"`      // 天气服务配置
	Notification NotificationConfig `yaml:"notification"` // 厨师通知配置
}

// ServerConfig HTTP 服务器配置。
type ServerConfig struct {
	Port int    `yaml:"port"` // 监听端口，默认 8080
	Mode string `yaml:"mode"` // Gin 运行模式 (debug/release/test)
}

// MySQLConfig 数据库连接配置。
type MySQLConfig struct {
	Host     string `yaml:"host"`     // 数据库主机地址
	Port     int    `yaml:"port"`     // 数据库端口
	User     string `yaml:"user"`     // 数据库用户名
	Password string `yaml:"password"` // 数据库密码
	Database string `yaml:"database"` // 数据库名称
}

// DSN 生成 MySQL 连接字符串（DSN），使用 utf8mb4 字符集并开启时间解析。
func (m MySQLConfig) DSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		m.User, m.Password, m.Host, m.Port, m.Database)
}

// JWTConfig JWT 令牌签发与验证配置。
type JWTConfig struct {
	Secret      string `yaml:"secret"`       // JWT 签名密钥
	ExpireHours int    `yaml:"expire_hours"` // Token 过期时间（小时）
}

// WeChatConfig 微信小程序配置。
type WeChatConfig struct {
	AppID            string `yaml:"appid"`             // 小程序 AppID
	Secret           string `yaml:"secret"`            // 小程序 AppSecret
	TemplateID       string `yaml:"template_id"`       // 订阅消息模板 ID
	MiniprogramState string `yaml:"miniprogram_state"` // 小程序版本：developer / trial / formal
}

// OSSConfig 阿里云 OSS 对象存储配置。
type OSSConfig struct {
	Endpoint        string `yaml:"endpoint"`          // OSS endpoint 地址
	AccessKeyID     string `yaml:"access_key_id"`     // AccessKey ID
	AccessKeySecret string `yaml:"access_key_secret"` // AccessKey Secret
	Bucket          string `yaml:"bucket"`            // OSS Bucket 名称
	Region          string `yaml:"region"`            // OSS 地域
	CustomDomain    string `yaml:"custom_domain"`     // 自定义域名（CDN），为空则使用默认 OSS 域名
}

// RedisConfig Redis 连接配置。
type RedisConfig struct {
	Addr     string `yaml:"addr"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
}

// WeatherConfig 天气服务配置（Open-Meteo）。
type WeatherConfig struct {
	Enabled       bool    `yaml:"enabled"`
	Provider      string  `yaml:"provider"`
	DefaultCity   string  `yaml:"default_city"`
	DefaultLat    float64 `yaml:"default_lat"`
	DefaultLon    float64 `yaml:"default_lon"`
	CacheTTLHours int     `yaml:"cache_ttl_hours"`
}

// AIRateLimitConfig AI 推荐限流。
type AIRateLimitConfig struct {
	Enabled      bool `yaml:"enabled"`
	MaxRequests  int  `yaml:"max_requests"`
	WindowHours  int  `yaml:"window_hours"`
}

// AIConfig AI 大模型服务配置（兼容 OpenAI API 格式）。
type AIConfig struct {
	APIKey                 string            `yaml:"api_key"`
	BaseURL                string            `yaml:"base_url"`
	Model                  string            `yaml:"model"`
	RecommendEnabled       bool              `yaml:"recommend_enabled"` // 是否开放 AI 推荐（前端入口 + API）
	RecommendCacheTTLHours int               `yaml:"recommend_cache_ttl_hours"`
	RecommendCount         int               `yaml:"recommend_count"`
	RateLimit              AIRateLimitConfig `yaml:"rate_limit"`
}

// NotificationConfig 厨师点菜通知总配置。
type NotificationConfig struct {
	Enabled          bool                   `yaml:"enabled"`
	Retry            NotificationRetry      `yaml:"retry"`
	WebSocket        NotificationWebSocket  `yaml:"websocket"`
	WeChatSubscribe  NotificationWxSub      `yaml:"wechat_subscribe"`
	WecomWorkbench   NotificationWecom      `yaml:"wecom_workbench"`
	ServerChan       NotificationServerChan `yaml:"server_chan"`
	Bark             NotificationBark       `yaml:"bark"`
	Ntfy             NotificationNtfy       `yaml:"ntfy"`
	Worker           NotificationWorker     `yaml:"worker"`
}

// NotificationRetry 外部通道重试策略。
type NotificationRetry struct {
	MaxAttempts   int   `yaml:"max_attempts"`
	IntervalsSec  []int `yaml:"intervals_sec"`
}

// NotificationWebSocket WebSocket 实时通知配置。
type NotificationWebSocket struct {
	Enabled          bool   `yaml:"enabled"`
	Path             string `yaml:"path"`
	PingIntervalSec  int    `yaml:"ping_interval_sec"`
	ReadTimeoutSec   int    `yaml:"read_timeout_sec"`
}

// NotificationWxSub 微信一次性订阅消息配置。
type NotificationWxSub struct {
	Enabled          bool   `yaml:"enabled"`
	TemplateID       string `yaml:"template_id"`
	MiniprogramState string `yaml:"miniprogram_state"`
	Page             string `yaml:"page"`
}

// NotificationWecom 企业微信微工作台应用消息配置。
type NotificationWecom struct {
	Enabled                bool   `yaml:"enabled"`
	CorpID                 string `yaml:"corp_id"`
	AgentID                int    `yaml:"agent_id"`
	Secret                 string `yaml:"secret"`
	APIBase                string `yaml:"api_base"`
	MsgType                string `yaml:"msg_type"`           // text / textcard / news（图文卡片，顶部展示菜品封面）
	CardURL                string `yaml:"card_url"`           // 卡片 H5 跳转地址（未配置小程序跳转时使用）
	MiniAppID              string `yaml:"mini_appid"`         // 卡片跳转小程序 AppID（空则回退 wechat.appid）
	MiniPagepath           string `yaml:"mini_pagepath"`      // 卡片跳转小程序页面路径，与 mini_appid 同时配置时优先于 card_url
	DefaultCoverURL        string `yaml:"default_cover_url"`  // 菜品无封面时的默认顶部图片（可选）
	DuplicateCheckInterval int    `yaml:"duplicate_check_interval"`
}

// NotificationServerChan Server酱配置。
type NotificationServerChan struct {
	Enabled bool   `yaml:"enabled"`
	APIBase string `yaml:"api_base"`
}

// NotificationBark Bark 推送配置。
type NotificationBark struct {
	Enabled         bool   `yaml:"enabled"`
	DefaultEndpoint string `yaml:"default_endpoint"`
}

// NotificationNtfy ntfy 推送配置。
type NotificationNtfy struct {
	Enabled         bool   `yaml:"enabled"`
	DefaultEndpoint string `yaml:"default_endpoint"`
	DefaultPriority string `yaml:"default_priority"`
	DefaultTags     string `yaml:"default_tags"`
}

// NotificationWorker delivery 重试 worker 配置。
type NotificationWorker struct {
	Enabled         bool `yaml:"enabled"`
	PollIntervalSec int  `yaml:"poll_interval_sec"`
}

// AIRecommendEnabled 是否开放 AI 推荐功能（前端入口与 /api/ai/* 接口）。
func (c *Config) AIRecommendEnabled() bool {
	if c == nil {
		return false
	}
	return c.AI.RecommendEnabled
}

// WeChatConfigured 小程序 AppID / Secret 是否已配置。
func (c *Config) WeChatConfigured() bool {
	if c == nil {
		return false
	}
	return strings.TrimSpace(c.WeChat.AppID) != "" && strings.TrimSpace(c.WeChat.Secret) != ""
}

// WeChatSubscribeConfigured 微信订阅消息通道是否具备发送条件。
func (c *Config) WeChatSubscribeConfigured() bool {
	if c == nil || !c.Notification.WeChatSubscribe.Enabled {
		return false
	}
	return c.WeChatConfigured() && c.EffectiveTemplateID() != ""
}

// EffectiveTemplateID 返回订阅消息模板 ID（notification 块优先，兼容 wechat 块）。
func (c *Config) EffectiveTemplateID() string {
	if c.Notification.WeChatSubscribe.TemplateID != "" {
		return c.Notification.WeChatSubscribe.TemplateID
	}
	return c.WeChat.TemplateID
}

// EffectiveWecomMiniAppID 返回企微卡片跳转小程序 AppID（wecom_workbench 块优先，回退 wechat.appid）。
func (c *Config) EffectiveWecomMiniAppID() string {
	if c == nil {
		return ""
	}
	if id := strings.TrimSpace(c.Notification.WecomWorkbench.MiniAppID); id != "" {
		return id
	}
	return strings.TrimSpace(c.WeChat.AppID)
}

// WecomMiniProgramJumpConfigured 是否已配置企微卡片跳转小程序（appid 与 pagepath 均非空）。
func (c *Config) WecomMiniProgramJumpConfigured() bool {
	if c == nil {
		return false
	}
	return c.EffectiveWecomMiniAppID() != "" && strings.TrimSpace(c.Notification.WecomWorkbench.MiniPagepath) != ""
}

// EffectiveMiniprogramState 返回小程序跳转版本。
func (c *Config) EffectiveMiniprogramState() string {
	if c.Notification.WeChatSubscribe.MiniprogramState != "" {
		return c.Notification.WeChatSubscribe.MiniprogramState
	}
	if c.WeChat.MiniprogramState != "" {
		return c.WeChat.MiniprogramState
	}
	return "developer"
}

// AppConfig 全局配置实例，在 Load 后可用。
var AppConfig *Config

// Load 从指定路径加载 YAML 配置文件，并设置默认值。
//
// 参数:
//   path - 配置文件路径（如 "config.yaml"）
//
// 返回值:
//   error - 读取或解析失败时返回错误
func Load(path string) error {
	// 读取配置文件内容
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read config: %w", err)
	}

	// 解析 YAML 到全局配置结构体
	AppConfig = &Config{}
	if err := yaml.Unmarshal(data, AppConfig); err != nil {
		return fmt.Errorf("parse config: %w", err)
	}

	// 设置默认端口
	if AppConfig.Server.Port == 0 {
		AppConfig.Server.Port = 8080
	}
	// 设置默认运行模式
	if AppConfig.Server.Mode == "" {
		AppConfig.Server.Mode = "debug"
	}
	applyNotificationDefaults(AppConfig)
	applyRedisWeatherAIDefaults(AppConfig)
	return nil
}

func applyRedisWeatherAIDefaults(c *Config) {
	if c.Redis.Addr == "" {
		c.Redis.Addr = "127.0.0.1:6379"
	}
	if c.Weather.DefaultCity == "" {
		c.Weather.DefaultCity = "成都"
	}
	if c.Weather.DefaultLat == 0 {
		c.Weather.DefaultLat = 30.5728
	}
	if c.Weather.DefaultLon == 0 {
		c.Weather.DefaultLon = 104.0668
	}
	if c.Weather.CacheTTLHours == 0 {
		c.Weather.CacheTTLHours = 3
	}
	if c.Weather.Provider == "" {
		c.Weather.Provider = "open_meteo"
	}
	if c.AI.RecommendCacheTTLHours == 0 {
		c.AI.RecommendCacheTTLHours = 24
	}
	if c.AI.RecommendCount == 0 {
		c.AI.RecommendCount = 5
	}
	if c.AI.RateLimit.MaxRequests == 0 {
		c.AI.RateLimit.MaxRequests = 3
	}
	if c.AI.RateLimit.WindowHours == 0 {
		c.AI.RateLimit.WindowHours = 3
	}
}

func applyNotificationDefaults(c *Config) {
	n := &c.Notification
	if n.WebSocket.Path == "" {
		n.WebSocket.Path = "/api/ws"
	}
	if n.WebSocket.PingIntervalSec == 0 {
		n.WebSocket.PingIntervalSec = 30
	}
	if n.WebSocket.ReadTimeoutSec == 0 {
		n.WebSocket.ReadTimeoutSec = 60
	}
	if n.Retry.MaxAttempts == 0 {
		n.Retry.MaxAttempts = 3
	}
	if len(n.Retry.IntervalsSec) == 0 {
		n.Retry.IntervalsSec = []int{60, 300, 900}
	}
	if n.WecomWorkbench.APIBase == "" {
		n.WecomWorkbench.APIBase = "https://qyapi.weixin.qq.com"
	}
	if n.WecomWorkbench.MsgType == "" {
		n.WecomWorkbench.MsgType = "news"
	}
	if n.WecomWorkbench.CardURL == "" {
		n.WecomWorkbench.CardURL = "https://www.zzzjc.xin"
	}
	if n.WecomWorkbench.DuplicateCheckInterval == 0 {
		n.WecomWorkbench.DuplicateCheckInterval = 1800
	}
	if n.ServerChan.APIBase == "" {
		n.ServerChan.APIBase = "https://sctapi.ftqq.com"
	}
	if n.Bark.DefaultEndpoint == "" {
		n.Bark.DefaultEndpoint = "https://api.day.app"
	}
	if n.Ntfy.DefaultEndpoint == "" {
		n.Ntfy.DefaultEndpoint = "https://ntfy.sh"
	}
	if n.Ntfy.DefaultPriority == "" {
		n.Ntfy.DefaultPriority = "high"
	}
	if n.Ntfy.DefaultTags == "" {
		n.Ntfy.DefaultTags = "cooking,food"
	}
	if n.Worker.PollIntervalSec == 0 {
		n.Worker.PollIntervalSec = 30
	}
	if n.WeChatSubscribe.Page == "" {
		n.WeChatSubscribe.Page = "pages/order/order"
	}
}
