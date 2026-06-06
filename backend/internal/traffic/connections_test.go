package traffic

import "testing"

func TestUserFromRule(t *testing.T) {
	cases := map[string]string{
		"auth_user=u1 => route(711proxy)": "u1",
		"auth_user=u42 => route(direct)":  "u42",
		"auth_user=[u7] => route(x)":      "u7", // tolerate a bracketed single user
		"final":                           "",
		"":                                "",
		"inbound=vless => route(direct)":  "",
	}
	for rule, want := range cases {
		if got := userFromRule(rule); got != want {
			t.Fatalf("userFromRule(%q)=%q, want %q", rule, got, want)
		}
	}
}
