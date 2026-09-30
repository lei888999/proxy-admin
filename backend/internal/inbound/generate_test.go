package inbound

import (
	"encoding/json"
	"strings"
	"testing"

	"singbox-admin/internal/models"
)

func TestGeneratePicksCredByProtocol(t *testing.T) {
	vd, _ := Get("vless-reality")
	vs, _ := vd.BuildSettings(nil)
	hd, _ := Get("hysteria2")
	hs, _ := hd.BuildSettings(nil)
	ob := uint(9)
	ins := []models.Inbound{
		{ID: 3, Tag: "v1", Type: "vless-reality", Network: "tcp", Port: 8443, Settings: vs,
			Users: []models.User{{ID: 7, Name: "a", UUID: "uuid-x", Password: "pw-x", OutboundID: &ob}}},
		{ID: 4, Tag: "h1", Type: "hysteria2", Network: "udp", Port: 443, Settings: hs,
			Users: []models.User{{ID: 8, Name: "b", UUID: "uuid-y", Password: "pw-y"}}}, // no outbound -> direct
	}
	outs := []models.Outbound{
		{ID: 9, Tag: "proxyA", Type: "socks5", Server: "1.2.3.4", Port: 1080, Username: "u", Password: "p"},
	}
	exp := ExperimentalConfig{ClashAddr: "127.0.0.1:9090", ClashSecret: "sec"}
	out, err := Generate(ins, outs, exp)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if !strings.Contains(out, `"uuid": "uuid-x"`) || !strings.Contains(out, `"password": "pw-y"`) {
		t.Fatal("creds not emitted per protocol")
	}

	var cfg map[string]any
	if err := json.Unmarshal([]byte(out), &cfg); err != nil {
		t.Fatalf("invalid json: %v", err)
	}

	obs := cfg["outbounds"].([]any)
	var foundSocks bool
	for _, o := range obs {
		m := o.(map[string]any)
		if m["tag"] == "proxyA" {
			foundSocks = true
			if m["type"] != "socks" || m["server"] != "1.2.3.4" || m["username"] != "u" {
				t.Fatalf("socks outbound wrong: %v", m)
			}
		}
	}
	if !foundSocks {
		t.Fatal("proxyA outbound missing")
	}

	route := cfg["route"].(map[string]any)
	if route["final"] != "direct" {
		t.Fatalf("route.final=%v", route["final"])
	}
	// sing-box >= 1.12 requires a default domain resolver for dialing.
	if dr, ok := route["default_domain_resolver"].(map[string]any); !ok || dr["server"] != "local" {
		t.Fatalf("route.default_domain_resolver missing/wrong: %v", route["default_domain_resolver"])
	}
	// The server only chooses the user's assigned egress; destinations never
	// override that choice. Each rule still attributes traffic to auth_user.
	rules := route["rules"].([]any)
	if len(rules) != 2 {
		t.Fatalf("want one egress rule per user: %v", rules)
	}
	for i, expected := range []string{"proxyA", "direct"} {
		rule := rules[i].(map[string]any)
		if len(rule) != 2 || rule["outbound"] != expected || len(rule["auth_user"].([]any)) != 1 {
			t.Fatalf("unexpected user egress: %v", rule)
		}
	}
	if _, ok := route["rule_set"]; ok {
		t.Fatal("server must not download geographic rule sets")
	}
	for _, forbidden := range []string{"ip_is_private", "domain_suffix", "geosite", "geoip", "rule_set"} {
		if strings.Contains(out, forbidden) {
			t.Fatalf("server must not include destination routing: %s", forbidden)
		}
	}

	dns := cfg["dns"].(map[string]any)
	if dns["final"] != "local" {
		t.Fatalf("dns.final=%v", dns["final"])
	}
	if _, legacy := dns["strategy"]; legacy {
		t.Fatal("dns.strategy is the deprecated legacy field; must not be emitted")
	}
	var foundLocal, foundDNS bool
	for _, s := range dns["servers"].([]any) {
		m := s.(map[string]any)
		if _, legacy := m["address"]; legacy {
			t.Fatalf("dns server uses the legacy 'address' field (fatal on sing-box >= 1.12): %v", m)
		}
		if m["tag"] == "local" && m["type"] == "local" {
			foundLocal = true
		}
		// New 1.12 DoT server format: type "tls" + server IP + detour.
		if m["tag"] == "dns-proxyA" && m["type"] == "tls" && m["server"] == "1.1.1.1" && m["detour"] == "proxyA" {
			foundDNS = true
		}
	}
	if !foundLocal {
		t.Fatal("local dns server (type:local) missing")
	}
	if !foundDNS {
		t.Fatal("dns detour server for proxyA missing or not in new tls format")
	}
	dnsRules := dns["rules"].([]any)
	if len(dnsRules) != 1 || dnsRules[0].(map[string]any)["server"] != "dns-proxyA" {
		t.Fatalf("DNS should follow the user egress without geographic overrides: %v", dnsRules)
	}

	experimental := cfg["experimental"].(map[string]any)
	cache := experimental["cache_file"].(map[string]any)
	if cache["enabled"] != true || cache["store_fakeip"] != true || cache["path"] != "cache.db" {
		t.Fatalf("cache file must persist rule/fake-ip state: %v", cache)
	}
}
