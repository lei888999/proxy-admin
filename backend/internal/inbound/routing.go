package inbound

import (
	"errors"
	"fmt"
	"net/netip"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

var ErrInvalidRouteRule = errors.New("invalid route rule")

// RouteRule is an ordered Mihomo client rule, never a server outbound selector.
type RouteRule struct {
	Type      string        `json:"type"`
	Value     string        `json:"value"`
	Policy    string        `json:"policy"`
	NoResolve bool          `json:"noResolve,omitempty"`
	Provider  *RuleProvider `json:"provider,omitempty"`
	// Only read for compatibility with persisted domain-only rules.
	Domain   string `json:"domain,omitempty"`
	Outbound string `json:"outbound,omitempty"`
}

type RuleProvider struct {
	URL      string `json:"url"`
	Behavior string `json:"behavior"`
	Format   string `json:"format"`
}

var geminiDomains = []string{
	"gemini.google.com", "accounts.google.com", "generativelanguage.googleapis.com",
	"oauth2.googleapis.com", "ai.google.dev", "googleapis.com", "googleusercontent.com", "gstatic.com",
}

var routeDomainLabel = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$`)
var routeCategory = regexp.MustCompile(`^[a-zA-Z0-9_@!.-]+$`)
var providerName = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

func supportsNoResolve(typ string) bool {
	switch typ {
	case "IP-CIDR", "IP-CIDR6", "IP-ASN", "GEOIP":
		return true
	}
	return false
}

func normalizeRouteRules(rules []RouteRule) ([]RouteRule, error) {
	out := make([]RouteRule, 0, len(rules))
	providers := map[string]RuleProvider{}
	for i, rule := range rules {
		if rule.Type == "" && rule.Domain != "" {
			rule.Type, rule.Value, rule.Policy = "DOMAIN-SUFFIX", rule.Domain, "节点"
			if rule.Outbound == "" || rule.Outbound == "direct" {
				rule.Policy = "DIRECT"
			}
		}
		rule.Domain, rule.Outbound = "", ""
		rule.Type = strings.ToUpper(strings.TrimSpace(rule.Type))
		rule.Value = strings.TrimSpace(rule.Value)
		rule.Policy = strings.TrimSpace(rule.Policy)
		fail := func(reason string) ([]RouteRule, error) {
			return nil, fmt.Errorf("%w: 第 %d 条规则：%s", ErrInvalidRouteRule, i+1, reason)
		}
		switch rule.Policy {
		case "DIRECT", "REJECT", "节点", "自动选择", "故障切换":
		default:
			return fail("请选择客户端策略（直连、拒绝或代理组）")
		}
		if strings.ContainsAny(rule.Value, ",\r\n\x00") || (rule.Value == "" && rule.Type != "MATCH") {
			return fail("匹配值不能为空或包含逗号、换行")
		}
		if rule.NoResolve && !supportsNoResolve(rule.Type) {
			return fail("no-resolve 仅适用于目标 IP 类规则")
		}
		if rule.Provider != nil && rule.Type != "RULE-SET" {
			return fail("只有 RULE-SET 可以配置规则集")
		}
		switch rule.Type {
		case "DOMAIN", "DOMAIN-SUFFIX":
			rule.Value = strings.ToLower(rule.Value)
			if rule.Type == "DOMAIN-SUFFIX" {
				rule.Value = strings.TrimPrefix(strings.TrimPrefix(rule.Value, "*."), ".")
			}
			if len(rule.Value) > 253 {
				return fail("域名过长")
			}
			for _, label := range strings.Split(rule.Value, ".") {
				if !routeDomainLabel.MatchString(label) {
					return fail("域名格式不正确")
				}
			}
		case "DOMAIN-KEYWORD", "DOMAIN-WILDCARD", "PROCESS-NAME", "PROCESS-PATH", "PROCESS-NAME-WILDCARD", "PROCESS-PATH-WILDCARD":
		case "DOMAIN-REGEX", "PROCESS-NAME-REGEX", "PROCESS-PATH-REGEX":
			if _, err := regexp.Compile(rule.Value); err != nil {
				return fail("正则表达式不正确")
			}
		case "IP-CIDR", "IP-CIDR6", "SRC-IP-CIDR":
			prefix, err := netip.ParsePrefix(rule.Value)
			if err != nil {
				return fail("请填写有效 CIDR，例如 192.168.0.0/16")
			}
			rule.Value = prefix.Masked().String()
		case "GEOSITE", "GEOIP", "SRC-GEOIP":
			if !routeCategory.MatchString(rule.Value) {
				return fail("地理规则名称不正确")
			}
		case "IP-ASN", "SRC-IP-ASN", "UID", "DSCP":
			n, err := strconv.ParseUint(rule.Value, 10, 32)
			if err != nil || (rule.Type == "DSCP" && n > 63) {
				return fail("数值超出范围")
			}
		case "DST-PORT", "SRC-PORT", "IN-PORT":
			for _, part := range strings.Split(rule.Value, "/") {
				bounds := strings.Split(part, "-")
				if len(bounds) > 2 {
					return fail("端口格式不正确")
				}
				prev := 0
				for _, bound := range bounds {
					n, err := strconv.Atoi(bound)
					if err != nil || n < 1 || n > 65535 || n < prev {
						return fail("端口须为 1–65535，范围须递增；多段用 / 分隔")
					}
					prev = n
				}
			}
		case "NETWORK":
			rule.Value = strings.ToLower(rule.Value)
			if rule.Value != "tcp" && rule.Value != "udp" {
				return fail("网络只能为 tcp 或 udp")
			}
		case "MATCH":
			if rule.Value != "" || i != len(rules)-1 {
				return fail("MATCH 不需要匹配值，并且必须放在最后")
			}
		case "RULE-SET":
			if !providerName.MatchString(rule.Value) || rule.Provider == nil {
				return fail("请填写规则集名称和来源")
			}
			provider := *rule.Provider
			u, err := url.Parse(provider.URL)
			if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.Fragment != "" {
				return fail("规则集须为有效 HTTPS 地址")
			}
			if provider.Behavior != "domain" && provider.Behavior != "ipcidr" && provider.Behavior != "classical" {
				return fail("规则集行为须为 domain、ipcidr 或 classical")
			}
			if provider.Format != "yaml" && provider.Format != "text" && provider.Format != "mrs" {
				return fail("规则集格式须为 yaml、text 或 mrs")
			}
			if provider.Format == "mrs" && provider.Behavior == "classical" {
				return fail("MRS 不支持 classical 规则集")
			}
			if previous, ok := providers[rule.Value]; ok && previous != provider {
				return fail("同名规则集不能使用不同来源或格式")
			}
			providers[rule.Value] = provider
		default:
			return fail("不支持的规则类型：" + rule.Type)
		}
		out = append(out, rule)
	}
	return out, nil
}

func (r RouteRule) clashRule() string {
	if r.Type == "MATCH" {
		return "MATCH," + r.Policy
	}
	line := r.Type + "," + r.Value + "," + r.Policy
	if r.NoResolve {
		line += ",no-resolve"
	}
	return line
}
