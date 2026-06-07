// Package service - 微信公众号/小程序服务。
//
// 本文件实现微信生态的核心服务功能，包括：
//   - Access Token 获取与缓存：调用微信 API 获取 access_token，带内存缓存和过期管理
//   - 订阅消息推送：通过微信小程序订阅消息模板向厨师推送点菜通知
//
// 技术要点：
//   - access_token 使用 sync.Mutex 保护的全局变量缓存，避免每次调用都请求微信 API
//   - token 提前 5 分钟过期（安全余量），防止使用时恰好过期
//   - 消息推送时自动将英文餐次类型（breakfast/lunch/dinner）映射为中文名称
//   - 消息内容自动截断（使用 rune 级别截断，正确处理中文字符）
//   - 自定义 bytesReadCloser 实现 io.ReadCloser 接口，避免导入 bytes 包
//
// 依赖：
//   - config.AppConfig.WeChat（AppID、Secret、TemplateID）
//   - 微信 access_token API：GET https://api.weixin.qq.com/cgi-bin/token
//   - 微信订阅消息 API：POST https://api.weixin.qq.com/cgi-bin/message/subscribe/send
package service

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"recipe-server/config"
	"recipe-server/internal/model"

	"gorm.io/gorm"
)

// 全局 access_token 缓存变量。
// 使用 sync.Mutex 保证并发安全，tokenExpireAt 记录过期时间。
var (
	accessToken   string       // 缓存的 access_token
	tokenExpireAt time.Time    // token 过期时间
	tokenMu       sync.Mutex   // 保护并发读写的互斥锁
)

// GetAccessToken 获取微信公众号/小程序的全局唯一接口调用凭据 access_token，带内存缓存。
//
// 调用微信官方接口：
//
//	GET https://api.weixin.qq.com/cgi-bin/token?grant_type=client_credential&appid=APPID&secret=APPSECRET
//
// 参数:
//   - 无
//
// 返回值:
//   - string - 有效的 access_token（当日首次调用从微信获取，后续从缓存返回）
//   - error  - HTTP 请求失败、JSON 解析失败或微信返回错误码时返回错误
//
// 缓存策略:
//   - 首次请求：调用微信 API → 解析响应 → 缓存 token 和过期时间 → 返回
//   - 后续请求：若 token 存在且未过期（提前 5 分钟作为安全余量）→ 直接返回缓存
//   - 过期后：重新调用微信 API 获取新 token
//
// 并发安全:
//   - 使用 sync.Mutex 保护全局变量，同一时刻只有一个 goroutine 访问缓存
//
// 说明:
//   - 微信 access_token 有效期通常为 7200 秒（2 小时）
//   - 提前 5 分钟（300 秒）标记过期，避免在高并发时使用即将过期的 token
//   - 返回值在项目其他函数（如 SendOrderNotify）中通过 Authorization 头传递
func GetAccessToken() (string, error) {
	tokenMu.Lock()
	defer tokenMu.Unlock()

	// 检查缓存：如果 token 存在且未到过期时间（提前 5 分钟），直接返回
	if accessToken != "" && time.Now().Before(tokenExpireAt) {
		return accessToken, nil
	}

	// 缓存失效，请求微信 API 获取新 token
	url := fmt.Sprintf("https://api.weixin.qq.com/cgi-bin/token?grant_type=client_credential&appid=%s&secret=%s",
		config.AppConfig.WeChat.AppID, config.AppConfig.WeChat.Secret)

	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("get access_token: %w", err)
	}
	defer resp.Body.Close()

	// 读取并解析响应体
	body, _ := io.ReadAll(resp.Body)
	var result struct {
		AccessToken string `json:"access_token"` // 获取到的凭证
		ExpiresIn   int    `json:"expires_in"`   // 凭证有效时间（单位：秒）
		ErrCode     int    `json:"errcode"`      // 错误码
		ErrMsg      string `json:"errmsg"`       // 错误信息
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("parse access_token: %w", err)
	}

	// 检查微信业务错误
	if result.ErrCode != 0 {
		return "", fmt.Errorf("get access_token error: %s", result.ErrMsg)
	}

	// 缓存新 token 及过期时间（提前 5 分钟过期，安全余量）
	accessToken = result.AccessToken
	tokenExpireAt = time.Now().Add(time.Duration(result.ExpiresIn-300) * time.Second)

	return accessToken, nil
}

// templateData 微信订阅消息模板数据字段映射。
// 微信订阅消息要求 data 字段为 map[string]struct{value string} 格式。
// 详见：https://developers.weixin.qq.com/miniprogram/dev/OpenApiDoc/mp-message-management/subscribe-message/sendMessage.html
type templateData map[string]struct {
	Value string `json:"value"` // 模板字段的实际值
}

// SendOrderNotify 通过微信小程序订阅消息向厨师推送点菜通知。
//
// 调用微信官方接口：
//
//	POST https://api.weixin.qq.com/cgi-bin/message/subscribe/send?access_token=ACCESS_TOKEN
//
// 参数:
//   - openid     string - 接收通知的厨师微信 OpenID
//   - recipeName string - 被点的菜名
//   - adderName  string - 点菜人昵称
//   - mealType   string - 餐次类型（breakfast/lunch/dinner）
//   - date       string - 日期（YYYY-MM-DD）
//
// 返回值:
//   - error - 未配置模板 ID（跳过发送）、获取 token 失败、HTTP 请求失败或微信返回错误时返回错误
//
// 消息模板字段映射:
//   - time7   → 订单时间（日期 + 餐次中文名）
//   - thing14 → 下单人（点菜人昵称，最多 20 个字符）
//   - thing13 → 产品名称（菜名，最多 20 个字符）
//
// 流程:
//  1. 检查 TemplateID 是否配置，未配置则跳过（返回 nil）
//  2. 获取 access_token（自动从缓存获取或远程请求）
//  3. 将英文餐次映射为中文名称
//  4. 构造订阅消息 JSON
//  5. 发送 POST 请求到微信订阅消息 API
//  6. 检查响应中的 errcode
//
// 注意:
//   - 订阅消息需要用户在小程序端授权后方可推送
//   - 消息内容字段需符合微信模板定义（thing 类型最多 20 字符）
func SendOrderNotify(openid, recipeName, adderName, mealType, date string) error {
	if config.AppConfig == nil || !config.AppConfig.WeChatSubscribeConfigured() {
		return nil
	}
	if strings.TrimSpace(openid) == "" {
		return nil
	}

	// 获取 access_token（自动处理缓存）
	token, err := GetAccessToken()
	if err != nil {
		return fmt.Errorf("get token: %w", err)
	}

	// 餐次类型英文 → 中文映射
	mealMap := map[string]string{"breakfast": "早餐", "lunch": "午餐", "dinner": "晚餐"}
	mealName := mealMap[mealType]
	if mealName == "" {
		mealName = mealType // 未知类型保留原值
	}

	// 构造订阅消息字段（thing 类型字段最多 20 字符，超出自动截断）
	data := templateData{
		"time7":   {Value: date + " " + mealName},          // 订单时间 = 日期 + 餐次
		"thing14": {Value: truncate(adderName, 20)},         // 下单人昵称
		"thing13": {Value: truncate(recipeName, 20)},        // 被点的菜名
	}

	// 构造微信订阅消息请求体
	// 默认使用开发版，方便真机调试时直接收到通知
	state := config.AppConfig.WeChat.MiniprogramState
	if state == "" {
		state = "developer"
	}
	body, _ := json.Marshal(map[string]any{
		"touser":            openid,                                   // 接收者的 OpenID
		"template_id":       config.AppConfig.EffectiveTemplateID(),  // 订阅消息模板 ID
		"page":              "pages/order/order",                      // 点击消息跳转的小程序页面
		"miniprogram_state": state,                                    // 跳转小程序类型
		"lang":              "zh_CN",                                   // 语言
		"data":              data,                                      // 模板字段数据
	})

	// 发送 POST 请求到微信订阅消息 API
	url := fmt.Sprintf("https://api.weixin.qq.com/cgi-bin/message/subscribe/send?access_token=%s", token)
	resp, err := http.Post(url, "application/json", bytesReader(body))
	if err != nil {
		return fmt.Errorf("send notify: %w", err)
	}
	defer resp.Body.Close()

	// 解析响应体检查微信业务错误码
	respBody, _ := io.ReadAll(resp.Body)
	log.Printf("[微信API] subscribe/send → openid=%s template=%s response: %s",
		openid, config.AppConfig.EffectiveTemplateID(), string(respBody))
	var result struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	json.Unmarshal(respBody, &result)
	if result.ErrCode != 0 {
		return fmt.Errorf("send notify error [%d]: %s", result.ErrCode, result.ErrMsg)
	}

	return nil
}

// UpdateChefServiceCard 更新指定家庭所有厨师的服务卡片。
//
// 聚合今天各餐次的点菜情况，生成摘要文本，调用 /wxa/set_user_notify 更新每位厨师的卡片。
//
// 参数:
//   - db        *gorm.DB  - 数据库连接
//   - familyID  uint64    - 家庭 ID
//
// 说明:
//   - 查询今天所有餐次的点菜记录，按餐次分组统计
//   - 卡片摘要格式："早餐2道/午餐3道/晚餐5道"
//   - 未配置 TemplateID 时静默跳过
func UpdateChefServiceCard(db *gorm.DB, familyID uint64) {
	if config.AppConfig == nil || !config.AppConfig.WeChatSubscribeConfigured() {
		return
	}

	token, err := GetAccessToken()
	if err != nil {
		log.Printf("[服务卡片] 获取token失败: %v", err)
		return
	}

	today := time.Now().Format("2006-01-02")

	// 查询今日各餐次点菜数量
	type mealCount struct {
		MealType string
		Count    int64
	}
	var counts []mealCount
	db.Model(&model.DailyOrder{}).
		Select("meal_type, COUNT(*) as count").
		Where("family_id = ? AND date = ?", familyID, today).
		Group("meal_type").
		Find(&counts)

	// 查询最近点菜的菜名（最多3道）
	var recentOrders []model.DailyOrder
	db.Where("family_id = ? AND date = ?", familyID, today).
		Preload("Recipe").
		Order("created_at DESC").
		Limit(3).
		Find(&recentOrders)

	// 构建卡片的摘要文本
	mealMap := map[string]string{"breakfast": "早餐", "lunch": "午餐", "dinner": "晚餐"}
	var summaryParts []string
	totalCount := int64(0)
	for _, c := range counts {
		name := mealMap[c.MealType]
		if name == "" {
			name = c.MealType
		}
		summaryParts = append(summaryParts, fmt.Sprintf("%s%d道", name, c.Count))
		totalCount += c.Count
	}
	cardSummary := strings.Join(summaryParts, "/")
	if cardSummary == "" {
		cardSummary = "暂无点菜"
	} else {
		cardSummary = fmt.Sprintf("共%d道 %s", totalCount, cardSummary)
	}

	// 构建最近菜名摘要
	var dishNames []string
	for _, o := range recentOrders {
		if o.Recipe != nil {
			dishNames = append(dishNames, o.Recipe.Name)
		}
	}
	dishSummary := strings.Join(dishNames, "、")
	if dishSummary == "" {
		dishSummary = "等待点菜中"
	}

	// 查找所有厨师
	var chefs []model.FamilyMember
	db.Where("family_id = ? AND is_chef = ?", familyID, true).
		Preload("User").
		Find(&chefs)

	state := config.AppConfig.WeChat.MiniprogramState
	if state == "" {
		state = "developer"
	}

	for _, chef := range chefs {
		if chef.User == nil || chef.User.OpenID == "" {
			continue
		}

		data := templateData{
			"time7":   {Value: today},
			"thing13": {Value: truncate(cardSummary, 20)},
			"thing14": {Value: truncate(dishSummary, 20)},
		}

		body, _ := json.Marshal(map[string]any{
			"touser":            chef.User.OpenID,
			"template_id":       config.AppConfig.WeChat.TemplateID,
			"page":              "pages/order/order",
			"miniprogram_state": state,
			"lang":              "zh_CN",
			"data":              data,
		})

		url := fmt.Sprintf("https://api.weixin.qq.com/wxa/set_user_notify?access_token=%s", token)
		resp, err := http.Post(url, "application/json", bytesReader(body))
		if err != nil {
			log.Printf("[服务卡片] 更新失败 → chef=%s: %v", chef.User.Nickname, err)
			continue
		}

		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		log.Printf("[服务卡片] set_user_notify → chef=%s summary=%s response: %s",
			chef.User.Nickname, cardSummary, string(respBody))
	}

	log.Printf("[服务卡片] 已更新家庭 %d 的 %d 位厨师卡片 | %s", familyID, len(chefs), cardSummary)
}

// ======================== 动态消息 ========================

// activityInfo 动态消息活动信息，记录 activity_id 对应的家庭和日期。
type activityInfo struct {
	FamilyID uint64
	Date     string
}

// activityStore 内存中的 activity_id 映射表。
// key: activity_id (string), value: activityInfo
// 动态消息有效期默认 24 小时，过期后自动失效无需清理。
var activityStore sync.Map

// StoreActivity 存储 activity_id 与家庭/日期的映射关系。
func StoreActivity(activityID string, familyID uint64, date string) {
	activityStore.Store(activityID, activityInfo{FamilyID: familyID, Date: date})
	log.Printf("[动态消息] 存储 activity=%s family=%d date=%s", activityID, familyID, date)
}

// CreateActivityID 调用微信 API 创建一个动态消息的 activity_id。
//
// 返回值：
//   - string - activity_id（24 小时内有效）
//   - error  - 请求失败时返回错误
func CreateActivityID() (string, error) {
	token, err := GetAccessToken()
	if err != nil {
		return "", fmt.Errorf("get token: %w", err)
	}

	url := fmt.Sprintf("https://api.weixin.qq.com/cgi-bin/message/wxopen/activityid/create?access_token=%s", token)
	resp, err := http.Post(url, "application/json", nil)
	if err != nil {
		return "", fmt.Errorf("create activity_id: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	log.Printf("[动态消息] create activity_id response: %s", string(respBody))

	var result struct {
		ErrCode    int    `json:"errcode"`
		ErrMsg     string `json:"errmsg"`
		ActivityID string `json:"activity_id"`
	}
	json.Unmarshal(respBody, &result)
	if result.ErrCode != 0 {
		return "", fmt.Errorf("create activity_id error [%d]: %s", result.ErrCode, result.ErrMsg)
	}

	return result.ActivityID, nil
}

// UpdateDynamicMessages 更新指定家庭/日期下所有活跃的动态消息卡片。
//
// 参数：
//   - db       *gorm.DB - 数据库连接
//   - familyID uint64   - 家庭 ID
//   - date     string   - 日期（YYYY-MM-DD）
//
// 说明：
//   - 遍历 activityStore，找到匹配 familyID+date 的所有 activity_id
//   - 查询当天各餐次已点菜数量作为 member_count
//   - 查询家庭成员数作为 room_limit
//   - 调用 updatablemsg/send 更新卡片内容
func UpdateDynamicMessages(db *gorm.DB, familyID uint64, date string) {
	if config.AppConfig == nil || !config.AppConfig.WeChatConfigured() {
		return
	}

	// 统计当天总点菜数量（各餐次合计）
	var totalOrders int64
	db.Model(&model.DailyOrder{}).
		Where("family_id = ? AND date = ?", familyID, date).
		Count(&totalOrders)

	// 统计家庭成员数作为 room_limit
	var memberCount int64
	db.Model(&model.FamilyMember{}).
		Where("family_id = ?", familyID).
		Count(&memberCount)

	token, err := GetAccessToken()
	if err != nil {
		log.Printf("[动态消息] 获取token失败: %v", err)
		return
	}

	// 遍历所有 activity_id，更新匹配的卡片
	updatedCount := 0
	activityStore.Range(func(key, value any) bool {
		info := value.(activityInfo)
		if info.FamilyID != familyID || info.Date != date {
			return true // 不匹配，继续遍历
		}

		activityID := key.(string)
		body, _ := json.Marshal(map[string]any{
			"activity_id":  activityID,
			"target_state": 0, // 0 = 进行中
			"template_info": map[string]any{
				"parameter_list": []map[string]string{
					{"name": "member_count", "value": fmt.Sprintf("%d", totalOrders)},
					{"name": "room_limit", "value": fmt.Sprintf("%d", memberCount)},
				},
			},
		})

		url := fmt.Sprintf("https://api.weixin.qq.com/cgi-bin/message/wxopen/updatablemsg/send?access_token=%s", token)
		resp, err := http.Post(url, "application/json", bytesReader(body))
		if err != nil {
			log.Printf("[动态消息] 更新失败 activity=%s: %v", activityID, err)
			return true
		}

		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		log.Printf("[动态消息] 更新 activity=%s family=%d date=%s orders=%d response: %s",
			activityID, familyID, date, totalOrders, string(respBody))
		updatedCount++
		return true
	})

	if updatedCount > 0 {
		log.Printf("[动态消息] 已更新 %d 个动态卡片 | family=%d date=%s orders=%d/%d",
			updatedCount, familyID, date, totalOrders, memberCount)
	}
}

// truncate 截断字符串到指定字符数（rune 级别），超出部分用 "..." 替换。
//
// 使用 rune 切片而非字节切片，确保中英文混排时按字符数（而非字节数）截断。
// 例如截断"你好世界abc"到 5 个字符 → "你好世界a..."（而非按字节截断导致乱码）。
//
// 参数:
//   - s   string - 原始字符串
//   - max int    - 最大字符数
//
// 返回值:
//   - string - 截断后的字符串（len <= max 时返回原值，超出则返回前 max 个字符 + "..."）
func truncate(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max]) + "..."
}

// bytesReader 创建一个基于字节切片的 io.ReadCloser，用于将 JSON 序列化结果直接作为 HTTP 请求体。
//
// 自定义实现而非使用 bytes.NewReader + io.NopCloser 的原因：
// bytes.NewReader 返回的是 io.ReadSeeker，不是 io.ReadCloser；
// io.NopCloser 会引入额外的包依赖和一层包装。
// 此处直接实现最小化的 ReadCloser 接口，代码更简洁。
//
// 参数:
//   - b []byte - 待读取的字节数据
//
// 返回值:
//   - *bytesReadCloser - 实现了 io.ReadCloser 的读取器
func bytesReader(b []byte) *bytesReadCloser {
	return &bytesReadCloser{b: b, pos: 0}
}

// bytesReadCloser 基于字节切片的 io.ReadCloser 实现。
// 支持 io.Reader 接口的分段读取，以及 io.Closer 的空实现。
type bytesReadCloser struct {
	b   []byte // 底层字节数据
	pos int    // 当前读取位置（字节偏移）
}

// Read 实现 io.Reader 接口，从底层字节切片中读取最多 len(p) 个字节。
//
// 参数:
//   - p []byte - 目标缓冲区
//
// 返回值:
//   - n   int   - 实际读取的字节数
//   - err error - 读取完毕时返回 io.EOF
func (r *bytesReadCloser) Read(p []byte) (n int, err error) {
	if r.pos >= len(r.b) {
		return 0, io.EOF // 已读取全部数据
	}
	n = copy(p, r.b[r.pos:])
	r.pos += n
	return n, nil
}

// Close 实现 io.Closer 接口（空操作，字节切片无需释放资源）。
func (r *bytesReadCloser) Close() error { return nil }
