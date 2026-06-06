package traffic

import (
	"context"
	"encoding/json"
	"net/http"
	"regexp"
	"time"
)

// authUserRe extracts the user key from a Clash-API connection's rule string,
// e.g. `auth_user=u7 => route(proxyA)` -> "u7". sing-box does not populate
// metadata.user, so the route rule is how we attribute a connection to a user.
var authUserRe = regexp.MustCompile(`auth_user=\[?(u\d+)`)

func userFromRule(rule string) string {
	if m := authUserRe.FindStringSubmatch(rule); m != nil {
		return m[1]
	}
	return ""
}

// Connection is one active Clash-API connection: cumulative bytes since it
// opened, plus the authenticated user (the inbound user name we set to "u<id>").
type Connection struct {
	ID   string
	User string
	Up   int64
	Down int64
}

// ConnectionsClient reads the Clash API connection table. Abstracted so the
// poller can be tested against a fake.
type ConnectionsClient interface {
	Connections(ctx context.Context) ([]Connection, error)
}

// clashClient talks to sing-box's clash_api external controller.
type clashClient struct {
	addr   string
	secret string
}

// NewClashClient builds a client for the clash_api external_controller address.
func NewClashClient(addr, secret string) ConnectionsClient {
	return &clashClient{addr: addr, secret: secret}
}

// clashConnections mirrors the subset of GET /connections we use. The user is
// taken from metadata.user when present, otherwise parsed from the rule string
// (sing-box leaves metadata.user empty but the rule carries `auth_user=u<id>`).
type clashConnections struct {
	Connections []struct {
		ID       string `json:"id"`
		Upload   int64  `json:"upload"`
		Download int64  `json:"download"`
		Rule     string `json:"rule"`
		Metadata struct {
			User string `json:"user"`
		} `json:"metadata"`
	} `json:"connections"`
}

func (c *clashClient) Connections(ctx context.Context) ([]Connection, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+c.addr+"/connections", nil)
	if err != nil {
		return nil, err
	}
	if c.secret != "" {
		req.Header.Set("Authorization", "Bearer "+c.secret)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var body clashConnections
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, err
	}
	out := make([]Connection, 0, len(body.Connections))
	for _, cn := range body.Connections {
		user := cn.Metadata.User
		if user == "" {
			user = userFromRule(cn.Rule)
		}
		out = append(out, Connection{ID: cn.ID, User: user, Up: cn.Upload, Down: cn.Download})
	}
	return out, nil
}
