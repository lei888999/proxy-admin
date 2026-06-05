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
	exp := ExperimentalConfig{ClashAddr: "127.0.0.1:9090", ClashSecret: "sec"}
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
		t.Fatal("inbound user name should be the stable key u7 (becomes clash metadata.user)")
	}
	var cfg map[string]any
	if err := json.Unmarshal([]byte(out), &cfg); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	expBlock, ok := cfg["experimental"].(map[string]any)
	if !ok {
		t.Fatal("missing experimental block")
	}
	clash, ok := expBlock["clash_api"].(map[string]any)
	if !ok || clash["external_controller"] != "127.0.0.1:9090" {
		t.Fatalf("clash_api missing/wrong: %v", expBlock["clash_api"])
	}
	// v2ray_api must NOT be emitted — it is absent from the official sing-box build.
	if _, ok := expBlock["v2ray_api"]; ok {
		t.Fatal("v2ray_api must not be present (breaks stock sing-box startup)")
	}
}
