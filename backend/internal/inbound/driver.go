package inbound

import "errors"

var ErrUnknownType = errors.New("unknown inbound type")

// Cred is one user's credential (uuid for vless, password for hysteria2).
type Cred struct {
	Name       string
	Credential string
}

// Driver encapsulates one protocol. It is decoupled from the DB model.
type Driver interface {
	Type() string
	Label() string
	Network() string
	DefaultPort() uint16
	BuildSettings(params map[string]any) (string, error) // generate secrets + apply params/defaults -> settings JSON
	BuildInbound(tag string, port uint16, settings string, users []Cred) (map[string]any, error)
	PublicInfo(settings string) (map[string]any, error)
	CredentialKind() string                                                // "uuid" | "password"
	UpdateSettings(existing string, params map[string]any) (string, error) // keep secrets, apply params
	ResetSecrets(existing string) (string, error)                          // new secrets, keep params
	ClashProxy(name, serverHost string, port uint16, settings string, cred Cred) (map[string]any, error)
}

type TypeInfo struct {
	Type        string `json:"type"`
	Label       string `json:"label"`
	Network     string `json:"network"`
	DefaultPort uint16 `json:"defaultPort"`
}

// ExperimentalConfig holds the sing-box experimental API endpoints the panel
// uses for traffic stats (clash_api = live, v2ray_api = cumulative).
type ExperimentalConfig struct {
	ClashAddr   string
	ClashSecret string
	V2RayAddr   string
}

var registry = map[string]Driver{}

func register(d Driver) { registry[d.Type()] = d }

func Get(t string) (Driver, bool) { d, ok := registry[t]; return d, ok }

// Types returns registered drivers in a stable order (for the UI dropdown).
func Types() []TypeInfo {
	out := []TypeInfo{}
	for _, t := range []string{"vless-reality", "hysteria2"} {
		if d, ok := registry[t]; ok {
			out = append(out, TypeInfo{Type: d.Type(), Label: d.Label(), Network: d.Network(), DefaultPort: d.DefaultPort()})
		}
	}
	return out
}
