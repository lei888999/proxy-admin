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

type fakeKeyGen struct{}

func (fakeKeyGen) RealityKeypair() (string, string, error) { return "PRIV", "PUB", nil }
func (fakeKeyGen) UUID() string                            { return "uuid-fixed" }
func (fakeKeyGen) ShortID() string                         { return "deadbeef" }

func newTestService(t *testing.T) (*Service, *fakeWriter) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("db: %v", err)
	}
	if err := db.AutoMigrate(&models.Inbound{}, &models.User{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	w := &fakeWriter{}
	return NewService(db, w, fakeKeyGen{}), w
}

func TestCreateInboundGeneratesKeysAndWritesConfig(t *testing.T) {
	s, w := newTestService(t)
	in, err := s.CreateInbound("vless-in", 443, "www.microsoft.com")
	if err != nil {
		t.Fatalf("CreateInbound: %v", err)
	}
	if in.RealityPublicKey != "PUB" || in.RealityShortID != "deadbeef" || in.ServerName != "www.microsoft.com" {
		t.Fatalf("inbound = %+v", in)
	}
	if w.calls == 0 || !strings.Contains(w.last, "vless-in") {
		t.Fatalf("config not regenerated: %q", w.last)
	}
}

func TestCreateInboundDuplicateTag(t *testing.T) {
	s, _ := newTestService(t)
	if _, err := s.CreateInbound("t", 1, "h"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateInbound("t", 2, "h"); err != ErrTagExists {
		t.Fatalf("err = %v, want ErrTagExists", err)
	}
}

func TestCreateUserAndCascadeDelete(t *testing.T) {
	s, w := newTestService(t)
	in, _ := s.CreateInbound("t", 1, "h")
	u, err := s.CreateUser(in.ID, "alice")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if u.UUID != "uuid-fixed" || !strings.Contains(w.last, "uuid-fixed") {
		t.Fatalf("user/config wrong: %+v / %q", u, w.last)
	}
	if err := s.DeleteInbound(in.ID); err != nil {
		t.Fatalf("DeleteInbound: %v", err)
	}
	us, _ := s.ListUsers(in.ID)
	if len(us) != 0 {
		t.Fatalf("users not cascade-deleted: %d", len(us))
	}
}

func TestCreateUserInboundNotFound(t *testing.T) {
	s, _ := newTestService(t)
	if _, err := s.CreateUser(999, "x"); err != ErrNotFound {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestCreateInboundWhitespaceTagRejected(t *testing.T) {
	s, _ := newTestService(t)
	if _, err := s.CreateInbound("   ", 443, "h"); err != ErrInvalidTag {
		t.Fatalf("err = %v, want ErrInvalidTag", err)
	}
}

func TestCreateInboundTrimsTag(t *testing.T) {
	s, _ := newTestService(t)
	in, err := s.CreateInbound("  v1  ", 443, "h")
	if err != nil {
		t.Fatalf("CreateInbound: %v", err)
	}
	if in.Tag != "v1" {
		t.Fatalf("tag = %q, want v1 (trimmed)", in.Tag)
	}
}

func TestCreateInboundPortInUse(t *testing.T) {
	s, _ := newTestService(t)
	if _, err := s.CreateInbound("a", 443, "h"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateInbound("b", 443, "h"); err != ErrPortInUse {
		t.Fatalf("err = %v, want ErrPortInUse", err)
	}
}

func TestDeleteInboundNotFound(t *testing.T) {
	s, _ := newTestService(t)
	if err := s.DeleteInbound(123); err != ErrNotFound {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}
