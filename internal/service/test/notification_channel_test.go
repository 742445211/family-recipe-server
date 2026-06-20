package service_test
import (
	"recipe-server/internal/service"
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

	svc := service.NewNotificationChannelService(db)
	targets := svc.GetEnabledTargets(user.ID, &user)

	if !service.HasUserChannel(targets, model.ChannelServerChan) {
		t.Error("已配置的 Server酱 应出现在 targets")
	}
	if service.HasUserChannel(targets, model.ChannelWecomWorkbench) {
		t.Error("未在通道表配置的企业微信不应自动投递")
	}
	if !service.HasUserChannel(targets, model.ChannelWeChatSubscribe) {
		t.Error("微信订阅在全局与用户 openid 齐全时应可用")
	}
}

type fakeWecomResolver struct {
	called bool
	userid string
	err    error
}

func (f *fakeWecomResolver) GetUseridByMobile(mobile string) (string, error) {
	f.called = true
	return f.userid, f.err
}

func TestIsChineseMobile(t *testing.T) {
	for _, s := range []string{"13800138000", "15912345678", "18612345678"} {
		if !service.IsChineseMobileForTest(s) {
			t.Fatalf("%q 应识别为中国手机号", s)
		}
	}
	for _, s := range []string{"zhangsan", "12345", "138001380001", "23800138000", "1380013800a", ""} {
		if service.IsChineseMobileForTest(s) {
			t.Fatalf("%q 不应识别为中国手机号", s)
		}
	}
}

func TestCreateWecomChannelResolvesMobile(t *testing.T) {
	db := testutil.SetupTestDB(t)
	config.AppConfig = &config.Config{}
	user := model.User{OpenID: "openid-wecom"}
	db.Create(&user)

	resolver := &fakeWecomResolver{userid: "zhangsan"}
	svc := service.NewNotificationChannelService(db)
	svc.SetWecomResolver(resolver)

	ch, err := svc.Create(user.ID, service.ChannelInput{Channel: model.ChannelWecomWorkbench, Secret: "13800138000"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if !resolver.called {
		t.Fatal("手机号输入应调用企业微信查询接口")
	}
	if ch.Secret != "zhangsan" {
		t.Fatalf("应保存解析出的 userid, got %q", ch.Secret)
	}
	var refreshed model.User
	db.First(&refreshed, user.ID)
	if refreshed.WecomUserid != "zhangsan" {
		t.Fatalf("users.wecom_userid 应同步为解析结果, got %q", refreshed.WecomUserid)
	}
}

func TestCreateWecomChannelKeepsUserid(t *testing.T) {
	db := testutil.SetupTestDB(t)
	config.AppConfig = &config.Config{}
	user := model.User{OpenID: "openid-wecom-2"}
	db.Create(&user)

	resolver := &fakeWecomResolver{userid: "should-not-be-used"}
	svc := service.NewNotificationChannelService(db)
	svc.SetWecomResolver(resolver)

	ch, err := svc.Create(user.ID, service.ChannelInput{Channel: model.ChannelWecomWorkbench, Secret: "lisi"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if resolver.called {
		t.Fatal("userid 字符串不应触发手机号查询")
	}
	if ch.Secret != "lisi" {
		t.Fatalf("应直接保存 userid, got %q", ch.Secret)
	}
}

func TestUpdateWecomChannelResolvesMobile(t *testing.T) {
	db := testutil.SetupTestDB(t)
	config.AppConfig = &config.Config{}
	user := model.User{OpenID: "openid-wecom-3"}
	db.Create(&user)
	ch := model.NotificationChannel{UserID: user.ID, Channel: model.ChannelWecomWorkbench, Enabled: true, Secret: "old"}
	db.Create(&ch)

	resolver := &fakeWecomResolver{userid: "wangwu"}
	svc := service.NewNotificationChannelService(db)
	svc.SetWecomResolver(resolver)

	if err := svc.Update(user.ID, ch.ID, service.ChannelInput{Secret: "13912345678"}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if !resolver.called {
		t.Fatal("更新手机号应调用查询接口")
	}
	var refreshed model.NotificationChannel
	db.First(&refreshed, ch.ID)
	if refreshed.Secret != "wangwu" {
		t.Fatalf("更新后应保存解析出的 userid, got %q", refreshed.Secret)
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

	targets := service.NewNotificationChannelService(db).GetEnabledTargets(user.ID, &user)
	if service.HasUserChannel(targets, model.ChannelWeChatSubscribe) {
		t.Error("未配置小程序凭据时不应投递微信订阅消息")
	}
}
