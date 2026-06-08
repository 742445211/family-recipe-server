package notifier

import "testing"

func TestMealName(t *testing.T) {
	cases := map[string]string{
		"breakfast": "早餐",
		"lunch":     "午餐",
		"dinner":    "晚餐",
		"snack":     "snack",
	}
	for in, want := range cases {
		if got := MealName(in); got != want {
			t.Fatalf("%s: got %q want %q", in, got, want)
		}
	}
}

func TestSkippedResult(t *testing.T) {
	r := SkippedResult("user offline", "abc***xyz")
	if r.Status != "skipped" || r.ErrorMessage != "user offline" || r.MaskedTarget != "abc***xyz" {
		t.Fatalf("unexpected: %+v", r)
	}
}
