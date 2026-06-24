// Package dateutil - 日期字符串规范化工具。
//
// FormatYMD 将多种常见格式统一为 YYYY-MM-DD，供点菜、通知、AI 上下文等模块复用。
package dateutil

import (
	"strings"
	"time"
)

const layoutYMD = "2006-01-02"

// FormatYMD 将日期字符串规范为 YYYY-MM-DD；无法解析时返回去空格后的原值。
func FormatYMD(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}
	if len(s) >= 10 && s[4] == '-' && s[7] == '-' {
		return s[:10]
	}
	layouts := []string{
		layoutYMD,
		time.RFC3339,
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02 15:04:05",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t.Format(layoutYMD)
		}
	}
	return s
}
