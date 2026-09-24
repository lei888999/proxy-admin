package inbound

import "encoding/json"

// tlsPasswordSettings is shared by the TCP/TLS password protocols. The
// certificate stays in the private settings blob and is never exposed in the
// inbound view or subscription.
type tlsPasswordSettings struct {
	ServerName string `json:"serverName"`
	CertPEM    string `json:"certPEM"`
	KeyPEM     string `json:"keyPEM"`
}

type trojan struct{}
type anytls struct{}

func init() {
	register(trojan{})
	register(anytls{})
}

func (trojan) Type() string           { return "trojan" }
func (trojan) Label() string          { return "Trojan" }
func (trojan) Network() string        { return "tcp" }
func (trojan) DefaultPort() uint16    { return 443 }
func (trojan) CredentialKind() string { return "password" }
func (trojan) Schema() []Field {
	return []Field{{Name: "serverName", Label: "SNI", Type: "text", Default: "www.cloudflare.com", Required: true}}
}

func (trojan) BuildSettings(params map[string]any) (string, error) {
	return buildTLSPasswordSettings(params, "www.cloudflare.com")
}

func (trojan) BuildInbound(tag string, port uint16, settings string, users []Cred) (map[string]any, error) {
	s, err := parseTLSPasswordSettings(settings)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"type": "trojan", "tag": tag, "listen": "::", "listen_port": port,
		"users": passwordUsers(users),
		"tls":   tlsBlock(s),
	}, nil
}

func (trojan) PublicInfo(settings string) (map[string]any, error) {
	s, err := parseTLSPasswordSettings(settings)
	if err != nil {
		return nil, err
	}
	return map[string]any{"serverName": s.ServerName, "insecure": true}, nil
}

func (trojan) UpdateSettings(existing string, params map[string]any) (string, error) {
	s, err := parseTLSPasswordSettings(existing)
	if err != nil {
		return "", err
	}
	if v, ok := params["serverName"].(string); ok && v != "" {
		s.ServerName = v
	}
	b, err := json.Marshal(s)
	return string(b), err
}

func (trojan) ResetSecrets(existing string) (string, error) {
	s, err := parseTLSPasswordSettings(existing)
	if err != nil {
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

func (trojan) ClashProxy(name, serverHost string, port uint16, settings string, cred Cred) (map[string]any, error) {
	s, err := parseTLSPasswordSettings(settings)
	if err != nil {
		return nil, err
	}
	return map[string]any{"name": name, "type": "trojan", "server": serverHost, "port": port, "password": cred.Credential, "sni": s.ServerName, "skip-cert-verify": true}, nil
}

func (anytls) Type() string           { return "anytls" }
func (anytls) Label() string          { return "AnyTLS" }
func (anytls) Network() string        { return "tcp" }
func (anytls) DefaultPort() uint16    { return 9443 }
func (anytls) CredentialKind() string { return "password" }
func (anytls) Schema() []Field {
	return []Field{{Name: "serverName", Label: "SNI", Type: "text", Default: "www.cloudflare.com", Required: true}}
}
func (anytls) BuildSettings(params map[string]any) (string, error) {
	return buildTLSPasswordSettings(params, "www.cloudflare.com")
}
func (anytls) BuildInbound(tag string, port uint16, settings string, users []Cred) (map[string]any, error) {
	s, err := parseTLSPasswordSettings(settings)
	if err != nil {
		return nil, err
	}
	return map[string]any{"type": "anytls", "tag": tag, "listen": "::", "listen_port": port, "users": passwordUsers(users), "tls": tlsBlock(s)}, nil
}
func (anytls) PublicInfo(settings string) (map[string]any, error) {
	s, err := parseTLSPasswordSettings(settings)
	if err != nil {
		return nil, err
	}
	return map[string]any{"serverName": s.ServerName, "insecure": true}, nil
}
func (anytls) UpdateSettings(existing string, params map[string]any) (string, error) {
	s, err := parseTLSPasswordSettings(existing)
	if err != nil {
		return "", err
	}
	if v, ok := params["serverName"].(string); ok && v != "" {
		s.ServerName = v
	}
	b, err := json.Marshal(s)
	return string(b), err
}
func (anytls) ResetSecrets(existing string) (string, error) { return trojan{}.ResetSecrets(existing) }
func (anytls) ClashProxy(name, serverHost string, port uint16, settings string, cred Cred) (map[string]any, error) {
	s, err := parseTLSPasswordSettings(settings)
	if err != nil {
		return nil, err
	}
	return map[string]any{"name": name, "type": "anytls", "server": serverHost, "port": port, "password": cred.Credential, "sni": s.ServerName, "skip-cert-verify": true}, nil
}

func buildTLSPasswordSettings(params map[string]any, defaultName string) (string, error) {
	name := defaultName
	if v, ok := params["serverName"].(string); ok && v != "" {
		name = v
	}
	cert, key, err := selfSignedCert(name)
	if err != nil {
		return "", err
	}
	b, err := json.Marshal(tlsPasswordSettings{ServerName: name, CertPEM: cert, KeyPEM: key})
	return string(b), err
}

func parseTLSPasswordSettings(raw string) (tlsPasswordSettings, error) {
	var s tlsPasswordSettings
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		return s, err
	}
	return s, nil
}

func passwordUsers(users []Cred) []map[string]any {
	out := make([]map[string]any, 0, len(users))
	for _, u := range users {
		out = append(out, map[string]any{"name": u.Name, "password": u.Credential})
	}
	return out
}

func tlsBlock(s tlsPasswordSettings) map[string]any {
	return map[string]any{"enabled": true, "server_name": s.ServerName, "certificate": []string{s.CertPEM}, "key": []string{s.KeyPEM}}
}
