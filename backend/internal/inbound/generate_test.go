package inbound

import (
	"encoding/json"
	"testing"

	"singbox-admin/internal/models"
)

func TestGenerateVlessReality(t *testing.T) {
	in := models.Inbound{
		Tag: "vless-in", Port: 443, Flow: "xtls-rprx-vision",
		RealityPrivateKey: "PRIV", RealityShortID: "deadbeef",
		Handshake: "www.microsoft.com", HandshakePort: 443, ServerName: "www.microsoft.com",
		Users: []models.User{{Name: "alice", UUID: "uuid-1"}},
	}
	out, err := Generate([]models.Inbound{in})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	var cfg map[string]any
	if err := json.Unmarshal([]byte(out), &cfg); err != nil {
		t.Fatalf("invalid json: %v\n%s", err, out)
	}
	ins := cfg["inbounds"].([]any)
	if len(ins) != 1 {
		t.Fatalf("inbounds len %d", len(ins))
	}
	ib := ins[0].(map[string]any)
	if ib["type"] != "vless" || ib["listen_port"].(float64) != 443 {
		t.Fatalf("inbound = %v", ib)
	}
	users := ib["users"].([]any)
	u := users[0].(map[string]any)
	if u["uuid"] != "uuid-1" || u["flow"] != "xtls-rprx-vision" {
		t.Fatalf("user = %v", u)
	}
	reality := ib["tls"].(map[string]any)["reality"].(map[string]any)
	if reality["private_key"] != "PRIV" {
		t.Fatalf("reality = %v", reality)
	}
	if reality["short_id"].([]any)[0] != "deadbeef" {
		t.Fatalf("short_id = %v", reality["short_id"])
	}
}

func TestGenerateEmptyInboundsIsValid(t *testing.T) {
	out, err := Generate(nil)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	var cfg map[string]any
	if err := json.Unmarshal([]byte(out), &cfg); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if _, ok := cfg["outbounds"]; !ok {
		t.Fatal("missing outbounds scaffold")
	}
}
