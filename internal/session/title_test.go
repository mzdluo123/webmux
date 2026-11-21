package session

import (
	"strings"
	"testing"
)

func TestAutomaticTitle(t *testing.T) {
	exact := strings.Repeat("长", 80)
	emoji := strings.Repeat("🚲", 78)
	for _, tc := range []struct{ input, want string }{
		{"  修复\n登录页的\t重定向问题 ", "修复 登录页的 重定向问题"},
		{"a\x1b[31mb\x07\xff", "a[31mb"},
		{" \n\t", ""},
		{exact, exact},
		{emoji + "🚲再写", emoji + "🚲…"},
		{emoji + " 再写", emoji + "…"},
	} {
		if got := AutomaticTitle(tc.input); got != tc.want {
			t.Errorf("AutomaticTitle(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}
