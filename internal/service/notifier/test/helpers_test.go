package notifier_test

import "recipe-server/internal/service/notifier"
import "testing"

func TestFormatIngredientsStringArray(t *testing.T) {
	got := notifier.FormatIngredients(`["盐","糖"]`)
	if got != "盐、糖" {
		t.Fatalf("got %q", got)
	}
}

func TestFormatIngredientsInvalidJSON(t *testing.T) {
	if notifier.FormatIngredients("{bad") != "" {
		t.Fatal("非法 JSON 应返回空")
	}
}

func TestTruncateRunes(t *testing.T) {
	if notifier.TruncateRunesForTest("你好世界", 10) != "你好世界" {
		t.Fatal("未超长不应截断")
	}
	if notifier.TruncateRunesForTest("你好世界abcdef", 4) != "你好世界..." {
		t.Fatalf("got %q", notifier.TruncateRunesForTest("你好世界abcdef", 4))
	}
}

func TestMaskSecret(t *testing.T) {
	if notifier.MaskForTest("short") != "***" {
		t.Fatal("短字符串应完全掩码")
	}
	if notifier.MaskForTest("abcdefghij") != "abc***hij" {
		t.Fatalf("got %q", notifier.MaskForTest("abcdefghij"))
	}
}

func TestTrimSlash(t *testing.T) {
	if notifier.TrimSlashForTest("https://api.test/") != "https://api.test" {
		t.Fatal(notifier.TrimSlashForTest("https://api.test/"))
	}
}

func TestBytesReader(t *testing.T) {
	r := notifier.BytesReaderForTest([]byte("hello"))
	buf := make([]byte, 3)
	n, err := r.Read(buf)
	if n != 3 || string(buf) != "hel" || err != nil {
		t.Fatalf("first read: n=%d buf=%q err=%v", n, buf, err)
	}
	n, err = r.Read(buf)
	if n != 2 || string(buf[:2]) != "lo" || err != nil {
		t.Fatalf("second read: n=%d err=%v", n, err)
	}
}
