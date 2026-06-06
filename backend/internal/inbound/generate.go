package inbound

import (
	"encoding/json"
	"fmt"
	"sort"

	"singbox-admin/internal/models"
)

// dnsResolver is the encrypted (DoT) upstream resolver used by per-outbound DNS
// servers so a proxied user's queries egress through their outbound.
const dnsResolver = "tls://1.1.1.1"

func Generate(inbounds []models.Inbound, outbounds []models.Outbound, exp ExperimentalConfig) (string, error) {
	tagByID := map[uint]string{}
	for _, o := range outbounds {
		tagByID[o.ID] = o.Tag
	}

	ins := []map[string]any{}
	userOutbound := map[uint]string{} // user id -> outbound tag
	for _, in := range inbounds {
		d, ok := Get(in.Type)
		if !ok {
			return "", fmt.Errorf("%w: %s", ErrUnknownType, in.Type)
		}
		kind := d.CredentialKind()
		creds := make([]Cred, 0, len(in.Users))
		for _, u := range in.Users {
			c := u.UUID
			if kind == "password" {
				c = u.Password
			}
			creds = append(creds, Cred{Name: fmt.Sprintf("u%d", u.ID), Credential: c})
			if u.OutboundID != nil {
				if tag, ok := tagByID[*u.OutboundID]; ok {
					userOutbound[u.ID] = tag
				}
			}
		}
		piece, err := d.BuildInbound(in.Tag, in.Port, in.Settings, creds)
		if err != nil {
			return "", err
		}
		ins = append(ins, piece)
	}

	obs := []map[string]any{{"type": "direct", "tag": "direct"}}
	for _, o := range outbounds {
		ob := map[string]any{"tag": o.Tag, "server": o.Server, "server_port": o.Port}
		if o.Type == "socks5" {
			ob["type"] = "socks" // sing-box's name for SOCKS5
		} else {
			ob["type"] = "http"
		}
		if o.Username != "" {
			ob["username"] = o.Username
		}
		if o.Password != "" {
			ob["password"] = o.Password
		}
		obs = append(obs, ob)
	}

	usersByTag := map[string][]string{}
	for uid, tag := range userOutbound {
		usersByTag[tag] = append(usersByTag[tag], fmt.Sprintf("u%d", uid))
	}
	tags := make([]string, 0, len(usersByTag))
	for tag := range usersByTag {
		tags = append(tags, tag)
	}
	sort.Strings(tags)

	routeRules := []map[string]any{}
	dnsServers := []map[string]any{{"tag": "local", "address": "local"}}
	dnsRules := []map[string]any{}
	for _, tag := range tags {
		users := usersByTag[tag]
		sort.Strings(users)
		routeRules = append(routeRules, map[string]any{"auth_user": users, "outbound": tag})
		dnsServers = append(dnsServers, map[string]any{"tag": "dns-" + tag, "address": dnsResolver, "detour": tag})
		dnsRules = append(dnsRules, map[string]any{"auth_user": users, "server": "dns-" + tag})
	}

	cfg := map[string]any{
		"log": map[string]any{"level": "info"},
		"dns": map[string]any{
			"servers":  dnsServers,
			"rules":    dnsRules,
			"final":    "local",
			"strategy": "prefer_ipv4",
		},
		"inbounds":  ins,
		"outbounds": obs,
		"route": map[string]any{
			"rules": routeRules,
			"final": "direct",
		},
		"experimental": map[string]any{
			"clash_api": map[string]any{
				"external_controller": exp.ClashAddr,
				"secret":              exp.ClashSecret,
			},
		},
	}
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}
