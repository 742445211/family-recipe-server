package dateutil_test

import "recipe-server/pkg/dateutil"
import "testing"

func TestFormatYMD(t *testing.T) {
	cases := map[string]string{
		"":                        "",
		"2026-06-08":              "2026-06-08",
		"2026-06-08T12:00:00Z":    "2026-06-08",
		"2026-06-08 15:04:05":     "2026-06-08",
		"  2026-06-08  ":          "2026-06-08",
		"invalid-date":            "invalid-date",
	}
	for in, want := range cases {
		if got := dateutil.FormatYMD(in); got != want {
			t.Fatalf("dateutil.FormatYMD(%q): got %q want %q", in, got, want)
		}
	}
}
