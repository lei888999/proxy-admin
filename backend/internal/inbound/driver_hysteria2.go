package inbound

import "encoding/json"

type hy2Settings struct {
	ServerName string `json:"serverName"`
	CertPEM    string `json:"certPEM"`
	KeyPEM     string `json:"keyPEM"`
	UpMbps     int    `json:"upMbps"`
	DownMbps   int    `json:"downMbps"`
}

type hysteria2 struct{}

func init() { register(hysteria2{}) }

func (hysteria2) Type() string        { return "hysteria2" }
func (hysteria2) Label() string       { return "Hysteria2" }
func (hysteria2) Network() string     { return "udp" }
func (hysteria2) DefaultPort() uint16 { return 443 }

func (hysteria2) BuildSettings(params map[string]any) (string, error) {
	sni := "bing.com"
	if v, ok := params["serverName"].(string); ok && v != "" {
		sni = v
	}
	cert, key, err := selfSignedCert(sni)
	if err != nil {
		return "", err
	}
	s := hy2Settings{ServerName: sni, CertPEM: cert, KeyPEM: key, UpMbps: 100, DownMbps: 100}
	if v, ok := toInt(params["upMbps"]); ok {
		s.UpMbps = v
	}
	if v, ok := toInt(params["downMbps"]); ok {
		s.DownMbps = v
	}
	b, err := json.Marshal(s)
	return string(b), err
}

func (hysteria2) BuildInbound(tag string, port uint16, settings string, users []Cred) (map[string]any, error) {
	var s hy2Settings
	if err := json.Unmarshal([]byte(settings), &s); err != nil {
		return nil, err
	}
	us := []map[string]any{}
	for _, u := range users {
		us = append(us, map[string]any{"name": u.Name, "password": u.Credential})
	}
	in := map[string]any{
		"type": "hysteria2", "tag": tag, "listen": "::", "listen_port": port,
		"users": us,
		"tls": map[string]any{
			"enabled": true, "server_name": s.ServerName, "alpn": []string{"h3"},
			"certificate": []string{s.CertPEM}, "key": []string{s.KeyPEM},
		},
	}
	if s.UpMbps > 0 {
		in["up_mbps"] = s.UpMbps
	}
	if s.DownMbps > 0 {
		in["down_mbps"] = s.DownMbps
	}
	return in, nil
}

func (hysteria2) PublicInfo(settings string) (map[string]any, error) {
	var s hy2Settings
	if err := json.Unmarshal([]byte(settings), &s); err != nil {
		return nil, err
	}
	return map[string]any{
		"serverName": s.ServerName, "upMbps": s.UpMbps, "downMbps": s.DownMbps, "insecure": true,
	}, nil
}

func toInt(v any) (int, bool) {
	switch n := v.(type) {
	case float64:
		return int(n), true
	case int:
		return n, true
	}
	return 0, false
}

func (hysteria2) CredentialKind() string { return "password" }

func (hysteria2) UpdateSettings(existing string, params map[string]any) (string, error) {
	var s hy2Settings
	if err := json.Unmarshal([]byte(existing), &s); err != nil {
		return "", err
	}
	if v, ok := params["serverName"].(string); ok && v != "" {
		s.ServerName = v
	}
	if v, ok := toInt(params["upMbps"]); ok {
		s.UpMbps = v
	}
	if v, ok := toInt(params["downMbps"]); ok {
		s.DownMbps = v
	}
	b, err := json.Marshal(s)
	return string(b), err
}

func (hysteria2) ClashProxy(name, serverHost string, port uint16, settings string, cred Cred) (map[string]any, error) {
	var s hy2Settings
	if err := json.Unmarshal([]byte(settings), &s); err != nil {
		return nil, err
	}
	return map[string]any{
		"name": name, "type": "hysteria2", "server": serverHost, "port": port,
		"password": cred.Credential, "sni": s.ServerName,
		"skip-cert-verify": true, "alpn": []string{"h3"},
	}, nil
}

func (hysteria2) ResetSecrets(existing string) (string, error) {
	var s hy2Settings
	if err := json.Unmarshal([]byte(existing), &s); err != nil {
		return "", err
	}
	cert, key, err := selfSignedCert(s.ServerName)
	if err != nil {
		return "", err
	}
	s.CertPEM, s.KeyPEM = cert, key
	b, err := json.Marshal(s)
	return string(b), err
}
