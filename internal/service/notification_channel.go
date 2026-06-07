package service

import (
	"errors"
	"strings"

	"recipe-server/config"
	"recipe-server/internal/model"

	"gorm.io/gorm"
)

// NotificationChannelService 用户通知通道配置。
type NotificationChannelService struct {
	db *gorm.DB
}

func NewNotificationChannelService(db *gorm.DB) *NotificationChannelService {
	return &NotificationChannelService{db: db}
}

// ListByUser 列出用户通道（脱敏）。
func (s *NotificationChannelService) ListByUser(userID uint64) ([]map[string]any, error) {
	var channels []model.NotificationChannel
	if err := s.db.Where("user_id = ?", userID).Find(&channels).Error; err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(channels))
	for _, ch := range channels {
		out = append(out, channelToResponse(ch))
	}
	return out, nil
}

func channelToResponse(ch model.NotificationChannel) map[string]any {
	masked := maskSecret(ch.Channel, ch.Secret, ch.Topic, ch.Endpoint)
	return map[string]any{
		"id":            ch.ID,
		"channel":       ch.Channel,
		"enabled":       ch.Enabled,
		"endpoint":      ch.Endpoint,
		"topic":         ch.Topic,
		"masked_target": masked,
		"created_at":    ch.CreatedAt,
		"updated_at":    ch.UpdatedAt,
	}
}

func maskSecret(channel, secret, topic, endpoint string) string {
	switch channel {
	case model.ChannelServerChan, model.ChannelBark, model.ChannelWecomWorkbench:
		if secret == "" {
			return ""
		}
		if len(secret) <= 6 {
			return "***"
		}
		return secret[:3] + "***" + secret[len(secret)-3:]
	case model.ChannelNtfy:
		if topic == "" {
			return ""
		}
		if len(topic) <= 4 {
			return "***"
		}
		return topic[:2] + "***" + topic[len(topic)-2:]
	default:
		return ""
	}
}

type ChannelInput struct {
	Channel  string
	Enabled  *bool
	Endpoint string
	Secret   string
	Topic    string
}

// Create 创建通道。
func (s *NotificationChannelService) Create(userID uint64, in ChannelInput) (*model.NotificationChannel, error) {
	if err := validateChannelInput(in); err != nil {
		return nil, err
	}
	ch := model.NotificationChannel{
		UserID:   userID,
		Channel:  in.Channel,
		Enabled:  true,
		Endpoint: in.Endpoint,
		Secret:   in.Secret,
		Topic:    in.Topic,
	}
	if in.Enabled != nil {
		ch.Enabled = *in.Enabled
	}
	if err := s.db.Create(&ch).Error; err != nil {
		return nil, err
	}
	if in.Channel == model.ChannelWecomWorkbench && in.Secret != "" {
		s.db.Model(&model.User{}).Where("id = ?", userID).Update("wecom_userid", in.Secret)
	}
	return &ch, nil
}

// Update 更新通道。
func (s *NotificationChannelService) Update(userID, id uint64, in ChannelInput) error {
	var ch model.NotificationChannel
	if err := s.db.Where("id = ? AND user_id = ?", id, userID).First(&ch).Error; err != nil {
		return errors.New("通道不存在")
	}
	updates := map[string]any{}
	if in.Endpoint != "" {
		updates["endpoint"] = in.Endpoint
	}
	if in.Secret != "" {
		updates["secret"] = in.Secret
	}
	if in.Topic != "" {
		updates["topic"] = in.Topic
	}
	if in.Enabled != nil {
		updates["enabled"] = *in.Enabled
	}
	if len(updates) == 0 {
		return nil
	}
	if err := s.db.Model(&ch).Updates(updates).Error; err != nil {
		return err
	}
	if ch.Channel == model.ChannelWecomWorkbench && in.Secret != "" {
		s.db.Model(&model.User{}).Where("id = ?", userID).Update("wecom_userid", in.Secret)
	}
	return nil
}

// Delete 删除通道。
func (s *NotificationChannelService) Delete(userID, id uint64) error {
	res := s.db.Where("id = ? AND user_id = ?", id, userID).Delete(&model.NotificationChannel{})
	if res.RowsAffected == 0 {
		return errors.New("通道不存在")
	}
	return res.Error
}

func validateChannelInput(in ChannelInput) error {
	switch in.Channel {
	case model.ChannelServerChan:
		if in.Secret == "" {
			return errors.New("请填写 SendKey")
		}
	case model.ChannelBark:
		if in.Secret == "" {
			return errors.New("请填写 Device Key")
		}
	case model.ChannelNtfy:
		if in.Topic == "" {
			return errors.New("请填写 Topic")
		}
	case model.ChannelWecomWorkbench:
		if in.Secret == "" {
			return errors.New("请填写企业微信 UserID")
		}
	case model.ChannelWeChatSubscribe, model.ChannelWebSocket:
		return errors.New("该通道不支持手动配置")
	default:
		return errors.New("不支持的通道类型")
	}
	return nil
}

// GetEnabledTargets 获取用户已启用的通道目标。
func (s *NotificationChannelService) GetEnabledTargets(userID uint64, user *model.User) map[string]notifierTarget {
	var channels []model.NotificationChannel
	s.db.Where("user_id = ? AND enabled = ?", userID, true).Find(&channels)
	targets := make(map[string]notifierTarget)
	for _, ch := range channels {
		targets[ch.Channel] = notifierTarget{
			Secret:   ch.Secret,
			Endpoint: ch.Endpoint,
			Topic:    ch.Topic,
		}
	}
	if user != nil && config.AppConfig != nil && config.AppConfig.WeChatSubscribeConfigured() && user.OpenID != "" {
		targets[model.ChannelWeChatSubscribe] = notifierTarget{OpenID: user.OpenID}
	}
	return targets
}

type notifierTarget struct {
	OpenID      string
	WecomUserid string
	Secret      string
	Endpoint    string
	Topic       string
}

// HasUserChannel 用户是否配置了某通道。
func HasUserChannel(targets map[string]notifierTarget, channel string) bool {
	t, ok := targets[channel]
	if !ok {
		return false
	}
	switch channel {
	case model.ChannelServerChan, model.ChannelBark, model.ChannelWecomWorkbench:
		return strings.TrimSpace(t.Secret) != ""
	case model.ChannelNtfy:
		return strings.TrimSpace(t.Topic) != ""
	case model.ChannelWeChatSubscribe:
		return strings.TrimSpace(t.OpenID) != ""
	default:
		return false
	}
}
