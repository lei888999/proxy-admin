package inbound

import (
	"encoding/json"
	"testing"

	"singbox-admin/internal/models"
)

func TestGenerateMixedProtocols(t *testing.T) {
	vd, _ := Get("vless-reality")
	vs, _ := vd.BuildSettings(nil)
	hd, _ := Get("hysteria2")
	hs, _ := hd.BuildSettings(nil)
	ins := []models.Inbound{
		{Tag: "v1", Type: "vless-reality", Network: "tcp", Port: 8443, Settings: vs,
			Users: []models.User{{Name: "a", Credential: "uuid-1"}}},
		{Tag: "h1", Type: "hysteria2", Network: "udp", Port: 443, Settings: hs,
			Users: []models.User{{Name: "b", Credential: "pass-1"}}},
	}
	out, err := Generate(ins)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	var cfg map[string]any
	if err := json.Unmarshal([]byte(out), &cfg); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	arr := cfg["inbounds"].([]any)
	if len(arr) != 2 {
		t.Fatalf("inbounds=%d", len(arr))
	}
	if arr[0].(map[string]any)["type"] != "vless" || arr[1].(map[string]any)["type"] != "hysteria2" {
		t.Fatalf("types: %v / %v", arr[0], arr[1])
	}
}

func TestGenerateUnknownType(t *testing.T) {
	_, err := Generate([]models.Inbound{{Tag: "x", Type: "nope", Settings: "{}"}})
	if err == nil {
		t.Fatal("want error for unknown type")
	}
}
