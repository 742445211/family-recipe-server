package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server ServerConfig `yaml:"server"`
	MySQL  MySQLConfig  `yaml:"mysql"`
	JWT    JWTConfig    `yaml:"jwt"`
	WeChat WeChatConfig `yaml:"wechat"`
	OSS    OSSConfig    `yaml:"oss"`
	AI     AIConfig     `yaml:"ai"`
}

type ServerConfig struct {
	Port int    `yaml:"port"`
	Mode string `yaml:"mode"`
}

type MySQLConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	Database string `yaml:"database"`
}

func (m MySQLConfig) DSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		m.User, m.Password, m.Host, m.Port, m.Database)
}

type JWTConfig struct {
	Secret      string `yaml:"secret"`
	ExpireHours int    `yaml:"expire_hours"`
}

type WeChatConfig struct {
	AppID      string `yaml:"appid"`
	Secret     string `yaml:"secret"`
	TemplateID string `yaml:"template_id"`
}

type OSSConfig struct {
	Endpoint        string `yaml:"endpoint"`
	AccessKeyID     string `yaml:"access_key_id"`
	AccessKeySecret string `yaml:"access_key_secret"`
	Bucket          string `yaml:"bucket"`
	Region          string `yaml:"region"`
	CustomDomain    string `yaml:"custom_domain"`
}

type AIConfig struct {
	APIKey  string `yaml:"api_key"`
	BaseURL string `yaml:"base_url"`
	Model   string `yaml:"model"`
}

var AppConfig *Config

func Load(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read config: %w", err)
	}
	AppConfig = &Config{}
	if err := yaml.Unmarshal(data, AppConfig); err != nil {
		return fmt.Errorf("parse config: %w", err)
	}
	if AppConfig.Server.Port == 0 {
		AppConfig.Server.Port = 8080
	}
	if AppConfig.Server.Mode == "" {
		AppConfig.Server.Mode = "debug"
	}
	return nil
}
