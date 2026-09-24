package inbound

import "errors"

var ErrUnknownType = errors.New("unknown inbound type")

// Cred is one user's credential (UUID or protocol password).
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
	Schema() []Field
}

// Field describes one protocol-specific setting rendered by the admin UI.
// Values are intentionally JSON-friendly so the same metadata drives both the
// API contract and the dynamic frontend form.
type Field struct {
	Name     string `json:"name"`
	Label    string `json:"label"`
	Type     string `json:"type"` // text | number
	Default  any    `json:"default,omitempty"`
	Required bool   `json:"required,omitempty"`
}

type TypeInfo struct {
	Type        string  `json:"type"`
	Label       string  `json:"label"`
	Network     string  `json:"network"`
	DefaultPort uint16  `json:"defaultPort"`
	Fields      []Field `json:"fields"`
}

// ExperimentalConfig holds the sing-box Clash API endpoint the panel uses for
// traffic stats (live throughput + per-user cumulative via /connections).
type ExperimentalConfig struct {
	ClashAddr   string
	ClashSecret string
}

var registry = map[string]Driver{}

func register(d Driver) { registry[d.Type()] = d }

func Get(t string) (Driver, bool) { d, ok := registry[t]; return d, ok }

// Types returns registered drivers in a stable order (for the UI dropdown).
func Types() []TypeInfo {
	out := []TypeInfo{}
	for _, t := range []string{"vless-reality", "hysteria2", "trojan", "anytls"} {
		if d, ok := registry[t]; ok {
			out = append(out, TypeInfo{Type: d.Type(), Label: d.Label(), Network: d.Network(), DefaultPort: d.DefaultPort(), Fields: d.Schema()})
		}
	}
	return out
}
