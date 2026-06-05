package inbound

import (
	"encoding/json"
	"strings"
	"testing"

	"singbox-admin/internal/models"
)

func TestGeneratePicksCredByProtocol(t *testing.T) {
	vd, _ := Get("vless-reality")
	vs, _ := vd.BuildSettings(nil)
	hd, _ := Get("hysteria2")
	hs, _ := hd.BuildSettings(nil)
	ins := []models.Inbound{
		{Tag: "v1", Type: "vless-reality", Network: "tcp", Port: 8443, Settings: vs,
			Users: []models.User{{Name: "a", UUID: "uuid-x", Password: "pw-x"}}},
		{Tag: "h1", Type: "hysteria2", Network: "udp", Port: 443, Settings: hs,
			Users: []models.User{{Name: "a", UUID: "uuid-x", Password: "pw-x"}}},
	}
	out, err := Generate(ins)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if !strings.Contains(out, `"uuid": "uuid-x"`) {
		t.Fatal("vless should use UUID")
	}
	if !strings.Contains(out, `"password": "pw-x"`) {
		t.Fatal("hysteria2 should use Password")
	}
	var cfg map[string]any
	if err := json.Unmarshal([]byte(out), &cfg); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
}
