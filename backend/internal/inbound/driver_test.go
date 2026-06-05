package inbound

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRegistryTypes(t *testing.T) {
	ts := Types()
	if len(ts) != 2 {
		t.Fatalf("types = %d, want 2", len(ts))
	}
	if ts[0].Type != "vless-reality" || ts[0].Network != "tcp" || ts[0].DefaultPort != 8443 {
		t.Fatalf("vless typeinfo: %+v", ts[0])
	}
	if ts[1].Type != "hysteria2" || ts[1].Network != "udp" || ts[1].DefaultPort != 443 {
		t.Fatalf("hy2 typeinfo: %+v", ts[1])
	}
}

func TestVlessDriver(t *testing.T) {
	d, ok := Get("vless-reality")
	if !ok {
		t.Fatal("vless driver not registered")
	}
	s, err := d.BuildSettings(map[string]any{"handshake": "example.com"})
	if err != nil {
		t.Fatalf("BuildSettings: %v", err)
	}
	in, err := d.BuildInbound("v1", 8443, s, []Cred{{Name: "a", Credential: "uuid-1"}})
	if err != nil {
		t.Fatalf("BuildInbound: %v", err)
	}
	if in["type"] != "vless" {
		t.Fatalf("type=%v", in["type"])
	}
	reality := in["tls"].(map[string]any)["reality"].(map[string]any)
	if reality["private_key"] == "" || reality["private_key"] == nil {
		t.Fatal("missing private_key")
	}
	users := in["users"].([]map[string]any)
	if users[0]["uuid"] != "uuid-1" {
		t.Fatalf("user uuid=%v", users[0]["uuid"])
	}
	pi, _ := d.PublicInfo(s)
	if pi["serverName"] != "example.com" {
		t.Fatalf("publicInfo serverName=%v", pi["serverName"])
	}
	if strings.Contains(toJSON(pi), "realityPrivateKey") {
		t.Fatal("publicInfo leaked private key")
	}
}

func TestHysteria2Driver(t *testing.T) {
	d, ok := Get("hysteria2")
	if !ok {
		t.Fatal("hy2 driver not registered")
	}
	s, err := d.BuildSettings(map[string]any{"serverName": "bing.com", "upMbps": float64(50), "downMbps": float64(200)})
	if err != nil {
		t.Fatalf("BuildSettings: %v", err)
	}
	in, err := d.BuildInbound("h1", 443, s, []Cred{{Name: "a", Credential: "pass1"}})
	if err != nil {
		t.Fatalf("BuildInbound: %v", err)
	}
	if in["type"] != "hysteria2" || in["up_mbps"] != 50 || in["down_mbps"] != 200 {
		t.Fatalf("hy2 inbound: %+v", in)
	}
	tls := in["tls"].(map[string]any)
	if tls["alpn"].([]string)[0] != "h3" {
		t.Fatalf("alpn=%v", tls["alpn"])
	}
	users := in["users"].([]map[string]any)
	if users[0]["password"] != "pass1" {
		t.Fatalf("user password=%v", users[0]["password"])
	}
	pi, _ := d.PublicInfo(s)
	if pi["insecure"] != true || pi["serverName"] != "bing.com" {
		t.Fatalf("publicInfo=%v", pi)
	}
	if strings.Contains(toJSON(pi), "keyPEM") {
		t.Fatal("publicInfo leaked key")
	}
}

func toJSON(v any) string { b, _ := json.Marshal(v); return string(b) }

func TestVlessUpdateAndReset(t *testing.T) {
	d, _ := Get("vless-reality")
	s, _ := d.BuildSettings(map[string]any{"handshake": "a.com"})
	u, err := d.UpdateSettings(s, map[string]any{"handshake": "b.com", "handshakePort": float64(8443)})
	if err != nil {
		t.Fatalf("UpdateSettings: %v", err)
	}
	pi, _ := d.PublicInfo(u)
	if pi["serverName"] != "b.com" {
		t.Fatalf("serverName=%v", pi["serverName"])
	}
	in1, _ := d.BuildInbound("t", 1, s, nil)
	in2, _ := d.BuildInbound("t", 1, u, nil)
	pk1 := in1["tls"].(map[string]any)["reality"].(map[string]any)["private_key"]
	pk2 := in2["tls"].(map[string]any)["reality"].(map[string]any)["private_key"]
	if pk1 != pk2 {
		t.Fatal("UpdateSettings must keep the private key")
	}
	r, err := d.ResetSecrets(u)
	if err != nil {
		t.Fatalf("ResetSecrets: %v", err)
	}
	in3, _ := d.BuildInbound("t", 1, r, nil)
	pk3 := in3["tls"].(map[string]any)["reality"].(map[string]any)["private_key"]
	if pk3 == pk2 {
		t.Fatal("ResetSecrets must change the key")
	}
	rpi, _ := d.PublicInfo(r)
	if rpi["serverName"] != "b.com" {
		t.Fatal("ResetSecrets must keep handshake")
	}
	if d.CredentialKind() != "uuid" {
		t.Fatalf("CredentialKind=%s", d.CredentialKind())
	}
}

func TestHy2UpdateAndReset(t *testing.T) {
	d, _ := Get("hysteria2")
	s, _ := d.BuildSettings(nil)
	u, err := d.UpdateSettings(s, map[string]any{"serverName": "x.com", "upMbps": float64(10), "downMbps": float64(20)})
	if err != nil {
		t.Fatalf("UpdateSettings: %v", err)
	}
	in, _ := d.BuildInbound("t", 1, u, nil)
	if in["up_mbps"] != 10 || in["down_mbps"] != 20 {
		t.Fatalf("up/down=%v/%v", in["up_mbps"], in["down_mbps"])
	}
	cert1 := in["tls"].(map[string]any)["certificate"]
	r, _ := d.ResetSecrets(u)
	in2, _ := d.BuildInbound("t", 1, r, nil)
	cert2 := in2["tls"].(map[string]any)["certificate"]
	if toJSON(cert1) == toJSON(cert2) {
		t.Fatal("ResetSecrets must change the cert")
	}
	rpi, _ := d.PublicInfo(r)
	if rpi["serverName"] != "x.com" || rpi["upMbps"] != 10 {
		t.Fatalf("reset publicInfo=%v", rpi)
	}
	if d.CredentialKind() != "password" {
		t.Fatalf("CredentialKind=%s", d.CredentialKind())
	}
}
