// Package notifier - 通知通道公共辅助函数（食材格式化、HTTP 响应读取等）。
package notifier

import (
	"encoding/json"
	"io"
	"strings"
)

type ingredientItem struct {
	Name   string `json:"name"`
	Amount string `json:"amount"`
}

// FormatIngredients 将菜谱 ingredients JSON 格式化为「番茄2个、鸡蛋3个」。
func FormatIngredients(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "[]" || raw == "null" {
		return ""
	}
	var items []ingredientItem
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		var strs []string
		if err2 := json.Unmarshal([]byte(raw), &strs); err2 != nil {
			return ""
		}
		return strings.Join(strs, "、")
	}
	parts := make([]string, 0, len(items))
	for _, it := range items {
		name := strings.TrimSpace(it.Name)
		if name == "" {
			continue
		}
		amount := strings.TrimSpace(it.Amount)
		if amount != "" {
			parts = append(parts, name+amount)
		} else {
			parts = append(parts, name)
		}
	}
	return strings.Join(parts, "、")
}

func truncateRunes(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max]) + "..."
}

func mask(s string) string {
	if len(s) <= 6 {
		return "***"
	}
	return s[:3] + "***" + s[len(s)-3:]
}

func trimSlash(s string) string {
	return strings.TrimRight(s, "/")
}

type bytesReadCloser struct {
	b   []byte
	pos int
}

func bytesReader(b []byte) *bytesReadCloser {
	return &bytesReadCloser{b: b}
}

func (r *bytesReadCloser) Read(p []byte) (int, error) {
	if r.pos >= len(r.b) {
		return 0, io.EOF
	}
	n := copy(p, r.b[r.pos:])
	r.pos += n
	return n, nil
}

func (r *bytesReadCloser) Close() error { return nil }
