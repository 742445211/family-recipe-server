package notifier

import "recipe-server/config"

// 以下符号仅供 internal/service/notifier/test 外部测试包访问。

func TruncateRunesForTest(s string, n int) string {
	return truncateRunes(s, n)
}

func MaskForTest(s string) string {
	return mask(s)
}

func TrimSlashForTest(s string) string {
	return trimSlash(s)
}

func BytesReaderForTest(b []byte) *bytesReadCloser {
	return bytesReader(b)
}

func RecipeCoverURLForTest(msg NotificationMessage, cfg config.NotificationWecom) string {
	return recipeCoverURL(msg, cfg)
}
