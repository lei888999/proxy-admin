package inbound

import (
	"encoding/json"

	"singbox-admin/internal/models"
)

// Generate builds a complete sing-box config from the given inbounds.
func Generate(inbounds []models.Inbound) (string, error) {
	type rUser struct {
		Name string `json:"name"`
		UUID string `json:"uuid"`
		Flow string `json:"flow"`
	}
	type handshake struct {
		Server     string `json:"server"`
		ServerPort uint16 `json:"server_port"`
	}
	type reality struct {
		Enabled    bool      `json:"enabled"`
		Handshake  handshake `json:"handshake"`
		PrivateKey string    `json:"private_key"`
		ShortID    []string  `json:"short_id"`
	}
	type tlsCfg struct {
		Enabled    bool    `json:"enabled"`
		ServerName string  `json:"server_name"`
		Reality    reality `json:"reality"`
	}
	type vlessIn struct {
		Type       string  `json:"type"`
		Tag        string  `json:"tag"`
		Listen     string  `json:"listen"`
		ListenPort uint16  `json:"listen_port"`
		Users      []rUser `json:"users"`
		TLS        tlsCfg  `json:"tls"`
	}
	type outbound struct {
		Type string `json:"type"`
		Tag  string `json:"tag"`
	}
	type logCfg struct {
		Level string `json:"level"`
	}
	type cfg struct {
		Log       logCfg     `json:"log"`
		Inbounds  []vlessIn  `json:"inbounds"`
		Outbounds []outbound `json:"outbounds"`
	}

	c := cfg{
		Log:       logCfg{Level: "info"},
		Inbounds:  []vlessIn{},
		Outbounds: []outbound{{Type: "direct", Tag: "direct"}},
	}
	for _, in := range inbounds {
		users := []rUser{}
		for _, u := range in.Users {
			users = append(users, rUser{Name: u.Name, UUID: u.UUID, Flow: in.Flow})
		}
		c.Inbounds = append(c.Inbounds, vlessIn{
			Type: "vless", Tag: in.Tag, Listen: "::", ListenPort: in.Port,
			Users: users,
			TLS: tlsCfg{
				Enabled:    true,
				ServerName: in.ServerName,
				Reality: reality{
					Enabled:    true,
					Handshake:  handshake{Server: in.Handshake, ServerPort: in.HandshakePort},
					PrivateKey: in.RealityPrivateKey,
					ShortID:    []string{in.RealityShortID},
				},
			},
		})
	}
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}
