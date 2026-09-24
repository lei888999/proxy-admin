package inbound

import (
	"net"

	"github.com/goccy/go-yaml"

	"singbox-admin/internal/models"
)

// UserSubscription returns a Clash-format YAML config for the user identified
// by subToken. serverHost is the public address clients dial.
func (s *Service) UserSubscription(subToken, serverHost string) (string, error) {
	if subToken == "" {
		return "", ErrNotFound
	}
	var u models.User
	if err := s.db.Preload("Inbounds").Where("sub_token = ?", subToken).First(&u).Error; err != nil {
		return "", ErrNotFound
	}

	proxies := []map[string]any{}
	names := []string{}
	for _, in := range u.Inbounds {
		d, ok := Get(in.Type)
		if !ok {
			continue
		}
		cred := Cred{Name: u.Name, Credential: u.UUID}
		if d.CredentialKind() == "password" {
			cred.Credential = u.Password
		}
		p, err := d.ClashProxy(in.Tag, serverHost, in.Port, in.Settings, cred)
		if err != nil {
			return "", err
		}
		proxies = append(proxies, p)
		names = append(names, in.Tag)
	}
	// A subscription with no usable inbound cannot proxy foreign traffic. Keep
	// the response explicit so clients do not silently import an empty node.
	if len(names) == 0 {
		return "", ErrNotFound
	}

	groupNames := append([]string(nil), names...)
	groups := []map[string]any{}
	if len(names) > 1 {
		groups = append(groups,
			map[string]any{
				"name": "自动选择", "type": "url-test", "proxies": names,
				"url": "https://www.gstatic.com/generate_204", "interval": 300,
			},
			map[string]any{
				"name": "故障切换", "type": "fallback", "proxies": names,
				"url": "https://www.gstatic.com/generate_204", "interval": 300,
			},
		)
		groupNames = append([]string{"自动选择", "故障切换"}, groupNames...)
	}
	groups = append(groups, map[string]any{"name": "节点", "type": "select", "proxies": groupNames})
	serverRule := "DOMAIN," + serverHost + ",DIRECT"
	if ip := net.ParseIP(serverHost); ip != nil {
		if ip4 := ip.To4(); ip4 != nil {
			serverRule = "IP-CIDR," + ip4.String() + "/32,DIRECT,no-resolve"
		} else {
			serverRule = "IP-CIDR6," + ip.String() + "/128,DIRECT,no-resolve"
		}
	}

	cfg := map[string]any{
		// Make imported profiles default to rule mode. If a client is switched to
		// Global mode from its UI, it intentionally ignores all rules below and
		// every request uses 节点, which looks exactly like failed China direct.
		"mode":         "rule",
		"proxies":      proxies,
		"proxy-groups": groups,
		// Keep DNS inside the client policy. Domestic answers use direct encrypted
		// resolvers; foreign fallback queries use the selected proxy group. IPv6 is
		// disabled in DNS; literal IPv6 is rejected by the rules below. Only an
		// active TUN/VPN can intercept applications that ignore system proxy settings.
		"dns": map[string]any{
			"enable":                  true,
			"ipv6":                    false,
			"enhanced-mode":           "fake-ip",
			"nameserver":              []string{"https://cloudflare-dns.com/dns-query#节点", "https://dns.google/dns-query#节点"},
			"nameserver-policy":       map[string]any{"geosite:cn": []string{"https://doh.pub/dns-query#DIRECT"}},
			"proxy-server-nameserver": []string{"https://doh.pub/dns-query#DIRECT"},
			"default-nameserver":      []string{"223.5.5.5"},
		},
		"ipv6": false,
		"tun": map[string]any{
			"enable": true, "stack": "mixed", "auto-route": true,
			"auto-detect-interface": true, "strict-route": true, "mtu": 1400,
			"dns-hijack":    []string{"any:53", "tcp://any:53"},
			"inet6-address": []string{"fdfe:dcba:9876::1/126"},
			"route-address": []string{"0.0.0.0/0", "::/0"},
		},
		// These are evaluated on the CLIENT, before traffic reaches the VPS.
		// GEOSITE requires Mihomo / Clash.Meta (which is already required for
		// this Hysteria2 subscription). GEOIP covers clients/apps that connect
		// to an IP directly OR whose domain is absent from geosite-cn but resolves
		// to a China IP. Do not use `no-resolve`: it prevents that latter fallback.
		"rules": []string{
			// The VPS endpoint must stay outside the tunnel. Otherwise a TUN client
			// sends its own panel/proxy connection back through that same proxy,
			// creating a loop that makes both the subscription and panel unreachable.
			serverRule,
			"IP-CIDR6,::/0,REJECT,no-resolve",
			"IP-CIDR,127.0.0.0/8,DIRECT,no-resolve",
			"IP-CIDR,10.0.0.0/8,DIRECT,no-resolve",
			"IP-CIDR,172.16.0.0/12,DIRECT,no-resolve",
			"IP-CIDR,192.168.0.0/16,DIRECT,no-resolve",
			"IP-CIDR,100.64.0.0/10,DIRECT,no-resolve",
			"IP-CIDR,169.254.0.0/16,DIRECT,no-resolve",
			"GEOSITE,CN,DIRECT",
			"GEOIP,CN,DIRECT",
			"MATCH,节点",
		},
	}
	b, err := yaml.Marshal(cfg)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
