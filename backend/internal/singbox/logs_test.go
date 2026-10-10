package singbox

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConnectionLogsCorrelateActualRoutes(t *testing.T) {
	dir := t.TempDir()
	logs := "\x1b[36mINFO\x1b[0m[0001] [123 0ms] inbound/vless[v1]: inbound connection from 192.0.2.1:1234\n" +
		"INFO[0001] [123 1ms] inbound/vless[v1]: [u7] inbound connection to api.example.com:443\n" +
		"INFO[0001] [456 0ms] inbound/trojan[t1]: [u8] inbound connection to [2001:db8::1]:443\n" +
		"INFO[0001] [123 2ms] outbound/http[美国出口]: outbound connection to api.example.com:443\n" +
		"INFO[0001] [456 1ms] outbound/direct[direct]: outbound connection to [2001:db8::1]:443\n" +
		"ERROR[0002] [123 1s] connection: open connection to api.example.com:443: proxy returned 502\n" +
		"INFO[0003] [789 0ms] inbound/vless[v1]: inbound connection from 192.0.2.2:1234\n" +
		"ERROR[0003] [789 1ms] inbound/vless[v1]: TLS handshake: invalid connection\n"
	os.WriteFile(filepath.Join(dir, "sing-box.log"), []byte(logs), 0600)
	s := New(nil, dir, "")
	result, err := s.ConnectionLogs()
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Entries) != 3 {
		t.Fatalf("entries: %+v", result)
	}
	failed, direct, proxy := result.Entries[0], result.Entries[1], result.Entries[2]
	if len(failed.Outbounds) != 0 || !strings.Contains(failed.Error, "handshake") {
		t.Fatalf("failed connection must not be labelled direct: %+v", failed)
	}
	if direct.Target != "[2001:db8::1]:443" || direct.Outbounds[0].Type != "direct" {
		t.Fatalf("direct: %+v", direct)
	}
	if proxy.User != "u7" || proxy.Source != "192.0.2.1:1234" || proxy.Target != "api.example.com:443" || proxy.Outbounds[0].Tag != "美国出口" || !strings.Contains(proxy.Error, "502") {
		t.Fatalf("proxy: %+v", proxy)
	}
	if strings.Contains(proxy.Time, "\x1b") {
		t.Fatal("ANSI not stripped")
	}
}

func TestConnectionLogsBoundedAndRestartSafe(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "sing-box.log")
	s := New(nil, dir, "")
	empty, err := s.ConnectionLogs()
	if err != nil || empty.Entries == nil || empty.Available {
		t.Fatalf("missing log: %+v %v", empty, err)
	}
	data := strings.Repeat("x", maxLogBytes+100) + "\nINFO[0001] [12 0ms] inbound/vless[v]: [u1] inbound connection to old.example:443\nINFO[0001] [12 1ms] outbound/http[old]: outbound connection to old.example:443\nINFO[0000] sing-box started (0.1s)\nINFO[0001] [12 0ms] inbound/vless[v]: [u2] inbound connection to new.example:443\n"
	os.WriteFile(p, []byte(data), 0600)
	got, err := s.ConnectionLogs()
	if err != nil {
		t.Fatal(err)
	}
	if !got.Truncated || len(got.Entries) != 2 || got.Entries[0].User != "u2" || len(got.Entries[0].Outbounds) != 0 {
		t.Fatalf("restart merged connections: %+v", got)
	}
	os.WriteFile(p, []byte("INFO[0001] [20 0ms] inbound/vless[v]: [u3] inbound connection to partial"), 0600)
	got, err = s.ConnectionLogs()
	if err != nil || len(got.Entries) != 0 {
		t.Fatal("must wait for complete log lines")
	}
}

func TestConnectionLogsKeepDistinctOutboundAttempts(t *testing.T) {
	logs := []byte("INFO[0001] [12 0ms] inbound/vless[v]: [u1] inbound connection to example.com:443\nINFO[0001] [12 0ms] outbound/http[proxy]: outbound connection to 1.1.1.1:853\nINFO[0001] [12 1ms] outbound/http[proxy]: outbound connection to example.com:443\n")
	entries := parseConnectionLogs(logs)
	if len(entries) != 1 || len(entries[0].Outbounds) != 2 || entries[0].Target != "example.com:443" {
		t.Fatalf("lost an outbound attempt: %+v", entries)
	}
}
