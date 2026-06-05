package inbound

import (
	"encoding/json"
	"fmt"

	"singbox-admin/internal/models"
)

// Generate builds a complete sing-box config from inbounds via the driver registry.
func Generate(inbounds []models.Inbound) (string, error) {
	ins := []map[string]any{}
	for _, in := range inbounds {
		d, ok := Get(in.Type)
		if !ok {
			return "", fmt.Errorf("%w: %s", ErrUnknownType, in.Type)
		}
		creds := make([]Cred, 0, len(in.Users))
		for _, u := range in.Users {
			creds = append(creds, Cred{Name: u.Name, Credential: u.Credential})
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
	}
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}
