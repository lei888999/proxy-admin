package inbound

import (
	"encoding/json"
	"fmt"

	"singbox-admin/internal/models"
)

func Generate(inbounds []models.Inbound, exp ExperimentalConfig) (string, error) {
	ins := []map[string]any{}
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
			// Stable per-user key (independent of display name); surfaces as the
			// Clash-API connection's metadata.user so the poller can attribute traffic.
			creds = append(creds, Cred{Name: fmt.Sprintf("u%d", u.ID), Credential: c})
		}
		piece, err := d.BuildInbound(in.Tag, in.Port, in.Settings, creds)
		if err != nil {
			return "", err
		}
		ins = append(ins, piece)
	}

	cfg := map[string]any{
		"log":       map[string]any{"level": "info"},
		"inbounds":  ins,
		"outbounds": []map[string]any{{"type": "direct", "tag": "direct"}},
		// Only clash_api: it ships in the official sing-box binary (v2ray_api does
		// not). Live throughput + per-user traffic both read from the Clash API.
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
