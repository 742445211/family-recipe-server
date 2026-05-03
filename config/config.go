// Package config 提供应用配置的加载与管理功能。
// 配置结构体映射 config.yaml 文件，包含服务器、数据库、JWT、
// 微信小程序、阿里云 OSS 以及 AI 服务的配置项。
package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config 全局应用配置，对应 config.yaml 顶层结构。
type Config struct {
	Server ServerConfig `yaml:"server"` // 服务器配置
	MySQL  MySQLConfig  `yaml:"mysql"`  // MySQL 数据库配置
	JWT    JWTConfig    `yaml:"jwt"`    // JWT 认证配置
	WeChat WeChatConfig `yaml:"wechat"` // 微信小程序配置
	OSS    OSSConfig    `yaml:"oss"`     // 阿里云 OSS 配置
	AI     AIConfig     `yaml:"ai"`      // AI 服务配置
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
	AppID      string `yaml:"appid"`       // 小程序 AppID
	Secret     string `yaml:"secret"`      // 小程序 AppSecret
	TemplateID string `yaml:"template_id"` // 订阅消息模板 ID
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

// AIConfig AI 大模型服务配置（兼容 OpenAI API 格式）。
type AIConfig struct {
	APIKey  string `yaml:"api_key"`  // API 密钥
	BaseURL string `yaml:"base_url"` // API 基础地址
	Model   string `yaml:"model"`    // 模型名称
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
	return nil
}
