package dateutil

import "testing"

func TestFormatYMD(t *testing.T) {
	cases := map[string]string{
		"2026-06-07":                    "2026-06-07",
		"2026-06-07T00:00:00+08:00":     "2026-06-07",
		"2026-06-07 15:30:00":           "2026-06-07",
		" 2026-06-07 ":                  "2026-06-07",
		"":                              "",
	}
	for in, want := range cases {
		if got := FormatYMD(in); got != want {
			t.Fatalf("FormatYMD(%q) = %q, want %q", in, got, want)
		}
	}
}
