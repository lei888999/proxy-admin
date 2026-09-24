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
	// The China rules must occur before the catch-all per-user outbound and all
	// rules must retain auth_user for connection-level traffic attribution.
	rules := route["rules"].([]any)
	var u7Private, u7China, u7Fallback, u8Direct bool
	for _, r := range rules {
		m := r.(map[string]any)
		au, ok := m["auth_user"].([]any)
		if !ok || len(au) != 1 {
			t.Fatalf("each route rule must target exactly one user, got %v", m["auth_user"])
		}
		switch au[0].(string) {
		case "u7":
			switch {
			case m["ip_is_private"] == true && m["outbound"] == "direct":
				u7Private = true
			case m["rule_set"] != nil && m["outbound"] == "direct":
				sets := m["rule_set"].([]any)
				if len(sets) != 2 || sets[0] != geositeCNTag || sets[1] != geoipCNTag {
					t.Fatalf("u7 China rule sets wrong: %v", sets)
				}
				u7China = true
			case m["outbound"] == "proxyA":
				u7Fallback = true
			}
		case "u8":
			u8Direct = m["outbound"] == "direct"
		}
	}
	if !u7Private || !u7China || !u7Fallback {
		t.Fatalf("u7 direct-China route chain missing: private=%v cn=%v fallback=%v", u7Private, u7China, u7Fallback)
	}
	if !u8Direct {
		t.Fatal("u8 (no outbound) should retain its explicit direct rule")
	}

	ruleSets := route["rule_set"].([]any)
	if len(ruleSets) != 2 {
		t.Fatalf("rule_set count=%d, want 2", len(ruleSets))
	}
	for _, raw := range ruleSets {
		rs := raw.(map[string]any)
		if rs["type"] != "remote" || rs["format"] != "binary" || rs["download_detour"] != "direct" {
			t.Fatalf("invalid remote rule set config: %v", rs)
		}
	}
	if ruleSets[0].(map[string]any)["tag"] != geositeCNTag || ruleSets[0].(map[string]any)["url"] != geositeCNURL {
		t.Fatalf("geosite rule set wrong: %v", ruleSets[0])
	}
	if ruleSets[1].(map[string]any)["tag"] != geoipCNTag || ruleSets[1].(map[string]any)["url"] != geoipCNURL {
		t.Fatalf("geoip rule set wrong: %v", ruleSets[1])
	}

	if len(rules) != 4 {
		t.Fatalf("route rule count=%d, want 4", len(rules))
	}
	if rules[0].(map[string]any)["ip_is_private"] != true || rules[1].(map[string]any)["rule_set"] == nil || rules[2].(map[string]any)["outbound"] != "proxyA" {
		t.Fatalf("u7 route order must be private -> China -> outbound, got %v", rules)
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
	if len(dnsRules) != 2 {
		t.Fatalf("dns rule count=%d, want 2: %v", len(dnsRules), dnsRules)
	}
	firstDNSRule := dnsRules[0].(map[string]any)
	if firstDNSRule["server"] != "local" {
		t.Fatalf("China DNS must use local/direct resolver: %v", firstDNSRule)
	}
	cnSets := firstDNSRule["rule_set"].([]any)
	if len(cnSets) != 1 || cnSets[0] != geositeCNTag {
		t.Fatalf("China DNS rule set wrong: %v", cnSets)
	}
	if dnsRules[1].(map[string]any)["server"] != "dns-proxyA" {
		t.Fatalf("per-user DNS rule wrong: %v", dnsRules[1])
	}
	experimental := cfg["experimental"].(map[string]any)
	cache := experimental["cache_file"].(map[string]any)
	if cache["enabled"] != true || cache["store_fakeip"] != true || cache["path"] != "cache.db" {
		t.Fatalf("cache file must persist rule/fake-ip state: %v", cache)
	}
}
