package service

import (
	"testing"

	"recipe-server/config"
	"recipe-server/internal/model"
	"recipe-server/internal/testutil"
)

func TestGetEnabledTargetsSkipsUnconfiguredChannels(t *testing.T) {
	db := testutil.SetupTestDB(t)
	config.AppConfig = &config.Config{
		Notification: config.NotificationConfig{
			WeChatSubscribe: config.NotificationWxSub{Enabled: true, TemplateID: "tmpl"},
		},
		WeChat: config.WeChatConfig{AppID: "wx-test", Secret: "secret"},
	}

	user := model.User{OpenID: "openid-1", WecomUserid: "wecom-user-1"}
	db.Create(&user)
	db.Create(&model.NotificationChannel{
		UserID: user.ID, Channel: model.ChannelServerChan, Enabled: true, Secret: "sendkey",
	})

	svc := NewNotificationChannelService(db)
	targets := svc.GetEnabledTargets(user.ID, &user)

	if !HasUserChannel(targets, model.ChannelServerChan) {
		t.Error("已配置的 Server酱 应出现在 targets")
	}
	if HasUserChannel(targets, model.ChannelWecomWorkbench) {
		t.Error("未在通道表配置的企业微信不应自动投递")
	}
	if !HasUserChannel(targets, model.ChannelWeChatSubscribe) {
		t.Error("微信订阅在全局与用户 openid 齐全时应可用")
	}
}

func TestGetEnabledTargetsSkipsWeChatWhenGlobalMissing(t *testing.T) {
	db := testutil.SetupTestDB(t)
	config.AppConfig = &config.Config{
		Notification: config.NotificationConfig{
			WeChatSubscribe: config.NotificationWxSub{Enabled: true, TemplateID: "tmpl"},
		},
	}

	user := model.User{OpenID: "openid-2"}
	db.Create(&user)

	targets := NewNotificationChannelService(db).GetEnabledTargets(user.ID, &user)
	if HasUserChannel(targets, model.ChannelWeChatSubscribe) {
		t.Error("未配置小程序凭据时不应投递微信订阅消息")
	}
}
