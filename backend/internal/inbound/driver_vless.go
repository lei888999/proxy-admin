package inbound

import "encoding/json"

type vlessSettings struct {
	RealityPrivateKey string `json:"realityPrivateKey"`
	RealityPublicKey  string `json:"realityPublicKey"`
	ShortID           string `json:"shortId"`
	Handshake         string `json:"handshake"`
	HandshakePort     uint16 `json:"handshakePort"`
	ServerName        string `json:"serverName"`
	Flow              string `json:"flow"`
}

type vlessReality struct{ kg KeyGen }

func init() { register(vlessReality{kg: NewKeyGen()}) }

func (vlessReality) Type() string        { return "vless-reality" }
func (vlessReality) Label() string       { return "VLESS-Reality" }
func (vlessReality) Network() string     { return "tcp" }
func (vlessReality) DefaultPort() uint16 { return 8443 }

func (d vlessReality) BuildSettings(params map[string]any) (string, error) {
	priv, pub, err := d.kg.RealityKeypair()
	if err != nil {
		return "", err
	}
	s := vlessSettings{
		RealityPrivateKey: priv, RealityPublicKey: pub, ShortID: d.kg.ShortID(),
		Handshake: "www.microsoft.com", HandshakePort: 443, Flow: "xtls-rprx-vision",
	}
	if v, ok := params["handshake"].(string); ok && v != "" {
		s.Handshake = v
	}
	s.ServerName = s.Handshake
	b, err := json.Marshal(s)
	return string(b), err
}

func (vlessReality) BuildInbound(tag string, port uint16, settings string, users []Cred) (map[string]any, error) {
	var s vlessSettings
	if err := json.Unmarshal([]byte(settings), &s); err != nil {
		return nil, err
	}
	us := []map[string]any{}
	for _, u := range users {
		us = append(us, map[string]any{"name": u.Name, "uuid": u.Credential, "flow": s.Flow})
	}
	return map[string]any{
		"type": "vless", "tag": tag, "listen": "::", "listen_port": port,
		"users": us,
		"tls": map[string]any{
			"enabled": true, "server_name": s.ServerName,
			"reality": map[string]any{
				"enabled":     true,
				"handshake":   map[string]any{"server": s.Handshake, "server_port": s.HandshakePort},
				"private_key": s.RealityPrivateKey,
				"short_id":    []string{s.ShortID},
			},
		},
	}, nil
}

func (vlessReality) PublicInfo(settings string) (map[string]any, error) {
	var s vlessSettings
	if err := json.Unmarshal([]byte(settings), &s); err != nil {
		return nil, err
	}
	return map[string]any{
		"realityPublicKey": s.RealityPublicKey, "shortId": s.ShortID,
		"serverName": s.ServerName, "handshakePort": s.HandshakePort, "flow": s.Flow,
	}, nil
}

func (vlessReality) CredentialKind() string { return "uuid" }

func (vlessReality) UpdateSettings(existing string, params map[string]any) (string, error) {
	var s vlessSettings
	if err := json.Unmarshal([]byte(existing), &s); err != nil {
		return "", err
	}
	if v, ok := params["handshake"].(string); ok && v != "" {
		s.Handshake = v
		s.ServerName = v
	}
	if v, ok := toInt(params["handshakePort"]); ok && v > 0 {
		s.HandshakePort = uint16(v)
	}
	b, err := json.Marshal(s)
	return string(b), err
}

func (d vlessReality) ResetSecrets(existing string) (string, error) {
	var s vlessSettings
	if err := json.Unmarshal([]byte(existing), &s); err != nil {
		return "", err
	}
	priv, pub, err := d.kg.RealityKeypair()
	if err != nil {
		return "", err
	}
	s.RealityPrivateKey, s.RealityPublicKey, s.ShortID = priv, pub, d.kg.ShortID()
	b, err := json.Marshal(s)
	return string(b), err
}
