package service

import (
	"io"
	"testing"
	"time"

	"recipe-server/config"
	"recipe-server/internal/model"
)

// 以下符号仅供 internal/service/test 外部测试包访问。

func TriggerOnConnectForTest(h *WebSocketHub, userID uint64) {
	h.triggerOnConnect(userID)
}

func WeatherCodeTextForTest(code int) string {
	return weatherCodeText(code)
}

func (s *NotificationService) PendingWebSocketNotificationsForTest(userID uint64) ([]model.Notification, error) {
	return s.pendingWebSocketNotifications(userID)
}

func (s *NotificationService) MarkWebSocketDeliverySentForTest(notificationID uint64) {
	s.markWebSocketDeliverySent(notificationID)
}

func IsChineseMobileForTest(s string) bool {
	return isChineseMobile(s)
}

func (s *ImageWorkerService) HandleTaskResultForTest(data []byte) {
	s.handleTaskResult(data)
}

func TaskResultOKForTest(status string) bool {
	return taskResultOK(status)
}

func TruncateForTest(s string, max int) string {
	return truncate(s, max)
}

func BytesReaderForTest(b []byte) *bytesReadCloser {
	return bytesReader(b)
}

func UploadToOSSForTest(cfg config.OSSConfig, key string, reader io.Reader, size int64, contentType string) (string, error) {
	return uploadToOSS(cfg, key, reader, size, contentType)
}

func SetAIServiceBaseURLForTest(s *AIService, url string) {
	s.baseURL = url
}

func SetWeatherAPIBaseForTest(w *WeatherService, url string) {
	w.apiBase = url
}

func WaitNotificationAsyncForTest(t *testing.T) {
	t.Helper()
	time.Sleep(200 * time.Millisecond)
}

func ParseAIRecommendJSONForTest(raw string) ([]AIRecommendItemInput, error) {
	return parseAIRecommendJSON(raw)
}

func AIItemKeyForTest(itemID string) string {
	return aiItemKey(itemID)
}

func FilterNewDishesOnlyForTest(inputs []AIRecommendItemInput, existing map[string]uint64) []AIRecommendItemInput {
	return filterNewDishesOnly(inputs, existing)
}
