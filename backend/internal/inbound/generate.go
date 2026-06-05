package inbound

import (
	"encoding/json"
	"fmt"
	"sort"

	"singbox-admin/internal/models"
)

func Generate(inbounds []models.Inbound, exp ExperimentalConfig) (string, error) {
	ins := []map[string]any{}
	tags := []string{}
	userSet := map[uint]struct{}{}
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
			// Stable per-user stats key (independent of display name).
			creds = append(creds, Cred{Name: fmt.Sprintf("u%d", u.ID), Credential: c})
			userSet[u.ID] = struct{}{}
		}
		piece, err := d.BuildInbound(in.Tag, in.Port, in.Settings, creds)
		if err != nil {
			return "", err
		}
		ins = append(ins, piece)
		tags = append(tags, in.Tag)
	}

	userKeys := make([]string, 0, len(userSet))
	for id := range userSet {
		userKeys = append(userKeys, fmt.Sprintf("u%d", id))
	}
	sort.Strings(userKeys)

	cfg := map[string]any{
		"log":       map[string]any{"level": "info"},
		"inbounds":  ins,
		"outbounds": []map[string]any{{"type": "direct", "tag": "direct"}},
		"experimental": map[string]any{
			"clash_api": map[string]any{
				"external_controller": exp.ClashAddr,
				"secret":              exp.ClashSecret,
			},
			"v2ray_api": map[string]any{
				"listen": exp.V2RayAddr,
				"stats": map[string]any{
					"enabled":  true,
					"inbounds": tags,
					"users":    userKeys,
				},
			},
		},
	}
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}
