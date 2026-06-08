package service

import "testing"

func TestTruncateRunes(t *testing.T) {
	if truncate("你好", 5) != "你好" {
		t.Fatal("短字符串不应截断")
	}
	if truncate("你好世界测试", 4) != "你好世界..." {
		t.Fatalf("got %q", truncate("你好世界测试", 4))
	}
}

func TestBytesReadCloser(t *testing.T) {
	r := bytesReader([]byte("data"))
	buf := make([]byte, 2)
	n, _ := r.Read(buf)
	if n != 2 || string(buf) != "da" {
		t.Fatalf("read: %d %q", n, buf)
	}
	if r.Close() != nil {
		t.Fatal("Close 应成功")
	}
}
