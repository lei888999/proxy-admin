package traffic

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

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

// clashConnections mirrors the subset of GET /connections we use. sing-box sets
// metadata.user to the inbound user's name for authenticated connections.
type clashConnections struct {
	Connections []struct {
		ID       string `json:"id"`
		Upload   int64  `json:"upload"`
		Download int64  `json:"download"`
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
		out = append(out, Connection{ID: cn.ID, User: cn.Metadata.User, Up: cn.Upload, Down: cn.Download})
	}
	return out, nil
}
