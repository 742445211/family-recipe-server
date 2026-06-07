package notifier

import (
	"io"
	"strings"
)

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
