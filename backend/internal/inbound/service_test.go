package inbound

import (
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"singbox-admin/internal/models"
)

type fakeWriter struct {
	last  string
	calls int
}

func (f *fakeWriter) SaveConfig(c string) error { f.last = c; f.calls++; return nil }

func newTestService(t *testing.T) (*Service, *fakeWriter) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("db: %v", err)
	}
	if err := db.AutoMigrate(&models.Inbound{}, &models.User{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	w := &fakeWriter{}
	return NewService(db, w), w
}

func TestCreateInboundVless(t *testing.T) {
	s, w := newTestService(t)
	in, err := s.CreateInbound("vless-reality", "v1", 8443, map[string]any{"handshake": "example.com"})
	if err != nil {
		t.Fatalf("CreateInbound: %v", err)
	}
	if in.Type != "vless-reality" || in.Network != "tcp" || in.Settings == "" {
		t.Fatalf("inbound = %+v", in)
	}
	if w.calls == 0 || !strings.Contains(w.last, "\"vless\"") {
		t.Fatalf("config not regenerated: %q", w.last)
	}
}

func TestCreateInboundUnknownType(t *testing.T) {
	s, _ := newTestService(t)
	if _, err := s.CreateInbound("nope", "x", 1, nil); err != ErrUnknownType {
		t.Fatalf("err = %v, want ErrUnknownType", err)
	}
}

func TestCreateInboundWhitespaceTag(t *testing.T) {
	s, _ := newTestService(t)
	if _, err := s.CreateInbound("hysteria2", "   ", 443, nil); err != ErrInvalidTag {
		t.Fatalf("err = %v, want ErrInvalidTag", err)
	}
}

func TestPortConflictByNetwork(t *testing.T) {
	s, _ := newTestService(t)
	if _, err := s.CreateInbound("vless-reality", "v", 443, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateInbound("hysteria2", "h", 443, nil); err != nil {
		t.Fatalf("udp 443 should coexist with tcp 443: %v", err)
	}
	if _, err := s.CreateInbound("vless-reality", "v2", 443, nil); err != ErrPortInUse {
		t.Fatalf("err = %v, want ErrPortInUse", err)
	}
}

func TestCreateUserCredentialByType(t *testing.T) {
	s, _ := newTestService(t)
	hin, _ := s.CreateInbound("hysteria2", "h", 443, nil)
	u, err := s.CreateUser(hin.ID, "alice")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if u.Credential == "" {
		t.Fatal("empty credential")
	}
}

func TestListInboundViewsNoSettings(t *testing.T) {
	s, _ := newTestService(t)
	_, _ = s.CreateInbound("vless-reality", "v1", 8443, nil)
	views, err := s.ListInboundViews()
	if err != nil || len(views) != 1 {
		t.Fatalf("views err=%v n=%d", err, len(views))
	}
	if views[0].PublicInfo["realityPublicKey"] == nil {
		t.Fatal("missing publicInfo")
	}
	if strings.Contains(toJSON(views[0]), "realityPrivateKey") {
		t.Fatal("view leaked private key")
	}
}

func TestCascadeDelete(t *testing.T) {
	s, _ := newTestService(t)
	in, _ := s.CreateInbound("vless-reality", "v1", 8443, nil)
	_, _ = s.CreateUser(in.ID, "a")
	if err := s.DeleteInbound(in.ID); err != nil {
		t.Fatal(err)
	}
	us, _ := s.ListUsers(in.ID)
	if len(us) != 0 {
		t.Fatalf("users not cascade-deleted: %d", len(us))
	}
}
