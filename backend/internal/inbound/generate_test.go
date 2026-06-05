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
		{ID: 3, Tag: "v1", Type: "vless-reality", Network: "tcp", Port: 8443, Settings: vs,
			Users: []models.User{{ID: 7, Name: "a", UUID: "uuid-x", Password: "pw-x"}}},
		{ID: 4, Tag: "h1", Type: "hysteria2", Network: "udp", Port: 443, Settings: hs,
			Users: []models.User{{ID: 7, Name: "a", UUID: "uuid-x", Password: "pw-x"}}},
	}
	exp := ExperimentalConfig{ClashAddr: "127.0.0.1:9090", ClashSecret: "sec", V2RayAddr: "127.0.0.1:9091"}
	out, err := Generate(ins, exp)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if !strings.Contains(out, `"uuid": "uuid-x"`) {
		t.Fatal("vless should use UUID")
	}
	if !strings.Contains(out, `"password": "pw-x"`) {
		t.Fatal("hysteria2 should use Password")
	}
	if !strings.Contains(out, `"name": "u7"`) {
		t.Fatal("inbound user name should be the stable stats key u7")
	}
	var cfg map[string]any
	if err := json.Unmarshal([]byte(out), &cfg); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	expBlock, ok := cfg["experimental"].(map[string]any)
	if !ok {
		t.Fatal("missing experimental block")
	}
	if _, ok := expBlock["clash_api"].(map[string]any); !ok {
		t.Fatal("missing clash_api")
	}
	v2 := expBlock["v2ray_api"].(map[string]any)
	stats := v2["stats"].(map[string]any)
	if stats["enabled"] != true {
		t.Fatal("v2ray stats not enabled")
	}
	users := stats["users"].([]any)
	if len(users) != 1 || users[0] != "u7" {
		t.Fatalf("stats.users=%v, want [u7]", users)
	}
}
