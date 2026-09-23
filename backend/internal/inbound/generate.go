package inbound

import (
	"encoding/json"
	"fmt"
	"sort"

	"singbox-admin/internal/models"
)

// dnsResolverServer is the encrypted (DoT, type "tls") upstream resolver used by
// per-outbound DNS servers so a proxied user's queries egress through their
// outbound. An IP literal avoids a bootstrap-resolve loop.
const dnsResolverServer = "1.1.1.1"

// Official SagerNet binary rule sets. A user assigned an outbound first matches
// private destinations and China domains/IPs to direct, then uses the outbound
// for the remainder. Remote rule sets stay fresh without baking a large database
// into this small control-plane binary.
const (
	geositeCNTag = "geosite-cn"
	geoipCNTag   = "geoip-cn"

	geositeCNURL = "https://raw.githubusercontent.com/SagerNet/sing-geosite/rule-set/geosite-cn.srs"
	geoipCNURL   = "https://raw.githubusercontent.com/SagerNet/sing-geoip/rule-set/geoip-cn.srs"
)

func Generate(inbounds []models.Inbound, outbounds []models.Outbound, exp ExperimentalConfig) (string, error) {
	tagByID := map[uint]string{}
	for _, o := range outbounds {
		tagByID[o.ID] = o.Tag
	}

	ins := []map[string]any{}
	userOutbound := map[uint]string{} // user id -> outbound tag (assigned only)
	allUsers := map[uint]struct{}{}   // every user that appears on an inbound
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
			allUsers[u.ID] = struct{}{}
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

	// Every route rule keeps auth_user in it. In addition to selecting a user's
	// path, that makes Clash's connection rule string contain `auth_user=u<id>`,
	// which the traffic sampler needs for per-user attribution. Rule order is
	// significant: for users assigned an outbound, private + China matches must
	// run before the catch-all outbound rule.
	userIDs := make([]uint, 0, len(allUsers))
	for uid := range allUsers {
		userIDs = append(userIDs, uid)
	}
	sort.Slice(userIDs, func(i, j int) bool { return userIDs[i] < userIDs[j] })
	routeRules := []map[string]any{}
	for _, uid := range userIDs {
		authUser := []string{fmt.Sprintf("u%d", uid)}
		tag := userOutbound[uid]
		if tag == "" {
			// No assigned outbound has always meant direct; retain one explicit
			// user rule so its traffic remains attributable.
			routeRules = append(routeRules, map[string]any{"auth_user": authUser, "outbound": "direct"})
			continue
		}
		// Never proxy LAN/private destinations through a user's overseas
		// upstream. These rules deliberately precede the China rule and the
		// user's catch-all.
		routeRules = append(routeRules, map[string]any{
			"auth_user":     authUser,
			"ip_is_private": true,
			"outbound":      "direct",
		})
		// A rule_set array is OR-ed by sing-box: either a matching China domain
		// or China IP is direct. URL-bearing traffic can match geosite; clients
		// that connect to an IP are covered by geoip.
		routeRules = append(routeRules, map[string]any{
			"auth_user": authUser,
			"rule_set":  []string{geositeCNTag, geoipCNTag},
			"outbound":  "direct",
		})
		routeRules = append(routeRules, map[string]any{
			"auth_user": authUser,
			"outbound":  tag,
		})
	}

	// DNS detour servers/rules, grouped by outbound (assigned users only) so a
	// proxied user's DNS egresses through their outbound. sing-box >= 1.12 typed
	// server format ({address:...} is deprecated and fatal).
	usersByTag := map[string][]string{}
	for uid, tag := range userOutbound {
		usersByTag[tag] = append(usersByTag[tag], fmt.Sprintf("u%d", uid))
	}
	tags := make([]string, 0, len(usersByTag))
	for tag := range usersByTag {
		tags = append(tags, tag)
	}
	sort.Strings(tags)
	dnsServers := []map[string]any{{"type": "local", "tag": "local"}}
	// Resolve China domains locally/directly before the per-user foreign DNS
	// rules below. The route rule set handles the resulting traffic too; this
	// avoids sending domestic lookups through an overseas upstream unnecessarily.
	dnsRules := []map[string]any{{
		"rule_set": []string{geositeCNTag},
		"server":   "local",
	}}
	for _, tag := range tags {
		users := usersByTag[tag]
		sort.Strings(users)
		dnsServers = append(dnsServers, map[string]any{"type": "tls", "tag": "dns-" + tag, "server": dnsResolverServer, "detour": tag})
		dnsRules = append(dnsRules, map[string]any{"auth_user": users, "server": "dns-" + tag})
	}

	cfg := map[string]any{
		"log": map[string]any{"level": "info"},
		"dns": map[string]any{
			"servers": dnsServers,
			"rules":   dnsRules,
			"final":   "local",
		},
		"inbounds":  ins,
		"outbounds": obs,
		"route": map[string]any{
			"rule_set": []map[string]any{
				{
					"type":            "remote",
					"tag":             geositeCNTag,
					"format":          "binary",
					"url":             geositeCNURL,
					"download_detour": "direct",
					"update_interval": "7d",
				},
				{
					"type":            "remote",
					"tag":             geoipCNTag,
					"format":          "binary",
					"url":             geoipCNURL,
					"download_detour": "direct",
					"update_interval": "7d",
				},
			},
			"rules": routeRules,
			"final": "direct",
			// sing-box >= 1.12 requires a resolver for outbounds that dial by
			// domain; "local" resolves upstream/proxy server addresses.
			"default_domain_resolver": map[string]any{"server": "local"},
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
