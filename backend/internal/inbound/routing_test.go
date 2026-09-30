package inbound

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/goccy/go-yaml"
	"singbox-admin/internal/models"
)

func TestClientRulesPersistWithoutApplyingServerConfig(t *testing.T) {
	s, w := newTestService(t)
	in, err := s.CreateInbound("vless-reality", "v1", 4443, nil)
	if err != nil {
		t.Fatal(err)
	}
	u, err := s.CreateUser("alice", []uint{in.ID}, nil)
	if err != nil {
		t.Fatal(err)
	}
	calls, server := w.calls, w.last
	rules := []RouteRule{
		{Type: "DOMAIN", Value: "Example.COM", Policy: "REJECT"},
		{Type: "IP-CIDR", Value: "8.8.8.0/24", Policy: "节点", NoResolve: true},
		{Type: "PROCESS-NAME", Value: "curl", Policy: "自动选择"},
		{Type: "RULE-SET", Value: "ads", Policy: "REJECT", Provider: &RuleProvider{URL: "https://example.com/ads.yaml", Behavior: "domain", Format: "yaml"}},
		{Type: "MATCH", Policy: "故障切换"},
	}
	if err := s.SaveRouteRules(rules); err != nil {
		t.Fatal(err)
	}
	if w.calls != calls || w.last != server {
		t.Fatal("client rule save must not apply/restart sing-box")
	}
	sub, err := s.UserSubscription(u.SubToken, "vps.example.com")
	if err != nil {
		t.Fatal(err)
	}
	var cfg struct {
		Rules     []string                  `yaml:"rules"`
		Providers map[string]map[string]any `yaml:"rule-providers"`
		Groups    []map[string]any          `yaml:"proxy-groups"`
	}
	if err := yaml.Unmarshal([]byte(sub), &cfg); err != nil {
		t.Fatal(err)
	}
	expected := []string{"DOMAIN,example.com,REJECT", "IP-CIDR,8.8.8.0/24,节点,no-resolve", "PROCESS-NAME,curl,自动选择", "RULE-SET,ads,REJECT"}
	prev := -1
	for _, rule := range expected {
		index := strings.Index(sub, rule)
		if index < 0 || index <= prev {
			t.Fatalf("missing or unordered rule %s", rule)
		}
		prev = index
	}
	if cfg.Rules[0] != "DOMAIN,vps.example.com,DIRECT" || cfg.Rules[len(cfg.Rules)-1] != "MATCH,故障切换" {
		t.Fatalf("protection/fallback order: %v", cfg.Rules)
	}
	if strings.Count(sub, "MATCH,") != 1 {
		t.Fatal("duplicate MATCH")
	}
	if cfg.Providers["ads"]["url"] != "https://example.com/ads.yaml" || cfg.Providers["ads"]["proxy"] != "节点" {
		t.Fatalf("provider missing: %v", cfg.Providers)
	}
	if len(cfg.Groups) != 3 {
		t.Fatal("single-node subscriptions must define all selectable groups")
	}
	if err := s.Regenerate(); err != nil {
		t.Fatal(err)
	}
	if w.last != server {
		t.Fatal("client rules changed generated server configuration")
	}
}

func TestLegacyRulesMigrateToClientPolicies(t *testing.T) {
	s, _ := newTestService(t)
	raw := `[{"domain":"*.Example.COM","outbound":"direct"},{"domain":"google.com","outbound":"old-upstream"}]`
	if err := s.db.Save(&models.Meta{Key: metaRouteRules, Value: raw}).Error; err != nil {
		t.Fatal(err)
	}
	rules, err := s.RouteRules()
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) != 2 || rules[0].Type != "DOMAIN-SUFFIX" || rules[0].Value != "example.com" || rules[0].Policy != "DIRECT" || rules[1].Policy != "节点" {
		t.Fatalf("migration: %+v", rules)
	}
	if err := s.SaveRouteRules(rules); err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(rules)
	if strings.Contains(string(b), `"outbound"`) || strings.Contains(string(b), `"domain"`) {
		t.Fatal("legacy fields must not remain in new API")
	}
}

func TestValidateClientRuleTypes(t *testing.T) {
	valid := []RouteRule{
		{Type: "DOMAIN-SUFFIX", Value: "*.EXAMPLE.com"}, {Type: "DOMAIN-KEYWORD", Value: "google"},
		{Type: "DOMAIN-REGEX", Value: `^example[0-9]+\.com$`}, {Type: "DOMAIN-WILDCARD", Value: "*.example.com"},
		{Type: "GEOSITE", Value: "google"}, {Type: "GEOIP", Value: "CN", NoResolve: true},
		{Type: "IP-CIDR6", Value: "2001:db8::/32"}, {Type: "SRC-IP-CIDR", Value: "192.168.0.0/16"},
		{Type: "IP-ASN", Value: "13335", NoResolve: true}, {Type: "SRC-IP-ASN", Value: "13335"},
		{Type: "SRC-GEOIP", Value: "CN"}, {Type: "DST-PORT", Value: "80/443/8000-9000"},
		{Type: "SRC-PORT", Value: "1234"}, {Type: "IN-PORT", Value: "7890"},
		{Type: "PROCESS-NAME", Value: "chrome.exe"}, {Type: "PROCESS-PATH", Value: `C:\Program Files\Browser\browser.exe`},
		{Type: "PROCESS-NAME-REGEX", Value: "(?i)telegram"}, {Type: "PROCESS-PATH-REGEX", Value: ".*bin/curl"},
		{Type: "NETWORK", Value: "udp"}, {Type: "UID", Value: "1000"}, {Type: "DSCP", Value: "4"}, {Type: "MATCH"},
	}
	for _, rule := range valid {
		t.Run(rule.Type, func(t *testing.T) {
			rule.Policy = "节点"
			if _, err := normalizeRouteRules([]RouteRule{rule}); err != nil {
				t.Fatal(err)
			}
		})
	}
	invalid := []RouteRule{
		{Type: "UNKNOWN", Value: "x"}, {Type: "IP-CIDR", Value: "not-an-ip"}, {Type: "DOMAIN", Value: "https://example.com"},
		{Type: "DST-PORT", Value: "65536"}, {Type: "SRC-PORT", Value: "9000-8000"}, {Type: "NETWORK", Value: "icmp"},
		{Type: "DOMAIN-REGEX", Value: "["}, {Type: "DOMAIN", Value: "a.com\nMATCH,DIRECT"},
		{Type: "DOMAIN-KEYWORD", Value: "a,REJECT"}, {Type: "DOMAIN", Value: "a.com", NoResolve: true},
		{Type: "MATCH", Value: "unexpected"}, {Type: "RULE-SET", Value: "missing"},
		{Type: "RULE-SET", Value: "../bad", Provider: &RuleProvider{URL: "https://example.com/a", Behavior: "domain", Format: "yaml"}},
		{Type: "RULE-SET", Value: "bad", Provider: &RuleProvider{URL: "file:///tmp/a", Behavior: "domain", Format: "yaml"}},
		{Type: "RULE-SET", Value: "bad", Provider: &RuleProvider{URL: "https://example.com/a", Behavior: "classical", Format: "mrs"}},
	}
	for _, rule := range invalid {
		rule.Policy = "节点"
		if _, err := normalizeRouteRules([]RouteRule{rule}); !errors.Is(err, ErrInvalidRouteRule) {
			t.Fatalf("accepted invalid rule %+v: %v", rule, err)
		}
	}
	if _, err := normalizeRouteRules([]RouteRule{{Type: "DOMAIN", Value: "a.com", Policy: "server-upstream"}}); err == nil {
		t.Fatal("accepted server-only outbound")
	}
	if _, err := normalizeRouteRules([]RouteRule{{Type: "MATCH", Policy: "DIRECT"}, {Type: "DOMAIN", Value: "a.com", Policy: "节点"}}); err == nil {
		t.Fatal("MATCH must be last")
	}
}

func TestClientRoutingMigrationAppliesOnceAndRetriesFailures(t *testing.T) {
	s, w := newTestService(t)
	if err := s.MigrateClientRouting(); err != nil {
		t.Fatal(err)
	}
	if w.calls != 1 {
		t.Fatal("upgrade must regenerate the existing server config")
	}
	if err := s.MigrateClientRouting(); err != nil {
		t.Fatal(err)
	}
	if w.calls != 1 {
		t.Fatal("subsequent startups must not reapply")
	}
	broken, _ := newTestService(t)
	if err := broken.db.Create(&models.Inbound{Type: "missing-driver", Tag: "broken", Port: 4443}).Error; err != nil {
		t.Fatal(err)
	}
	if err := broken.MigrateClientRouting(); err == nil {
		t.Fatal("failed generation should be reported")
	}
	var count int64
	broken.db.Model(&models.Meta{}).Where("key = ?", metaClientRouting).Count(&count)
	if count != 0 {
		t.Fatal("failed migration must retry at next startup")
	}
}

func TestDeletingOldUpstreamDoesNotInvalidateClientRules(t *testing.T) {
	s, _ := newTestService(t)
	upstream, err := s.CreateOutbound("socks5", "old-upstream", "1.2.3.4", 1080, "", "")
	if err != nil {
		t.Fatal(err)
	}
	s.db.Save(&models.Meta{Key: metaRouteRules, Value: `[{"domain":"example.com","outbound":"old-upstream"}]`})
	if err := s.DeleteOutbound(upstream.ID); err != nil {
		t.Fatal(err)
	}
	rules, err := s.RouteRules()
	if err != nil || rules[0].Policy != "节点" {
		t.Fatalf("client rules depend on server upstream: %+v %v", rules, err)
	}
}
