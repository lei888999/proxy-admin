package diagnostics

import (
	"context"
	"net"
	"testing"

	"singbox-admin/internal/inbound"
)

func TestProbeOutboundsReportsReachabilityAndKeepsOrder(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()
	port := uint16(listener.Addr().(*net.TCPAddr).Port)
	dead, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("dead listen: %v", err)
	}
	deadPort := uint16(dead.Addr().(*net.TCPAddr).Port)
	dead.Close()
	go func() {
		conn, acceptErr := listener.Accept()
		if acceptErr == nil {
			_ = conn.Close()
		}
	}()

	probes := ProbeOutbounds(context.Background(), []inbound.OutboundView{
		{Tag: "reachable", Server: "127.0.0.1", Port: port},
		{Tag: "invalid", Server: "127.0.0.1", Port: deadPort},
	})
	if len(probes) != 2 || probes[0].Tag != "reachable" || probes[1].Tag != "invalid" {
		t.Fatalf("probe order=%+v", probes)
	}
	if !probes[0].Reachable || probes[0].LatencyMs < 0 {
		t.Fatalf("reachable probe=%+v", probes[0])
	}
	if probes[1].Reachable || probes[1].Error == "" {
		t.Fatalf("unreachable probe=%+v", probes[1])
	}
}
