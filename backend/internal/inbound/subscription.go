package inbound

import (
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

	cfg := map[string]any{
		"proxies": proxies,
		"proxy-groups": []map[string]any{
			{"name": "节点", "type": "select", "proxies": names},
		},
		// These are evaluated on the CLIENT, before traffic reaches the VPS.
		// GEOSITE requires Mihomo / Clash.Meta (which is already required for
		// this Hysteria2 subscription). GEOIP covers clients/apps that connect
		// to an IP directly rather than preserving the destination domain.
		"rules": []string{
			"GEOSITE,CN,DIRECT",
			"GEOIP,CN,DIRECT,no-resolve",
			"MATCH,节点",
		},
	}
	b, err := yaml.Marshal(cfg)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
