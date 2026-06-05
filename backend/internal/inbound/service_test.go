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

func TestCreateUserWithInbounds(t *testing.T) {
	s, w := newTestService(t)
	v, _ := s.CreateInbound("vless-reality", "v1", 8443, nil)
	h, _ := s.CreateInbound("hysteria2", "h1", 443, nil)
	u, err := s.CreateUser("alice", []uint{v.ID, h.ID})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if u.UUID == "" || u.Password == "" {
		t.Fatalf("creds empty: %+v", u)
	}
	if !strings.Contains(w.last, u.UUID) || !strings.Contains(w.last, u.Password) {
		t.Fatalf("config missing creds: %q", w.last)
	}
	views, _ := s.ListUserViews()
	if len(views) != 1 || len(views[0].InboundIDs) != 2 {
		t.Fatalf("view=%+v", views)
	}
}

func TestUpdateUserReplacesInbounds(t *testing.T) {
	s, _ := newTestService(t)
	v, _ := s.CreateInbound("vless-reality", "v1", 8443, nil)
	h, _ := s.CreateInbound("hysteria2", "h1", 443, nil)
	u, _ := s.CreateUser("a", []uint{v.ID})
	if _, err := s.UpdateUser(u.ID, "a2", []uint{h.ID}); err != nil {
		t.Fatal(err)
	}
	views, _ := s.ListUserViews()
	if views[0].Name != "a2" || len(views[0].InboundIDs) != 1 || views[0].InboundIDs[0] != h.ID {
		t.Fatalf("view=%+v", views[0])
	}
}

func TestDeleteUserKeepsInbound(t *testing.T) {
	s, _ := newTestService(t)
	v, _ := s.CreateInbound("vless-reality", "v1", 8443, nil)
	u, _ := s.CreateUser("a", []uint{v.ID})
	if err := s.DeleteUser(u.ID); err != nil {
		t.Fatal(err)
	}
	var nUsers, nInbounds int64
	s.db.Model(&models.User{}).Count(&nUsers)
	s.db.Model(&models.Inbound{}).Count(&nInbounds)
	if nUsers != 0 || nInbounds != 1 {
		t.Fatalf("users=%d inbounds=%d", nUsers, nInbounds)
	}
}

func TestResetUserCreds(t *testing.T) {
	s, _ := newTestService(t)
	u, _ := s.CreateUser("a", nil)
	old := u.UUID
	r, _ := s.ResetUserCreds(u.ID)
	if r.UUID == old {
		t.Fatal("uuid not reset")
	}
}

func TestUpdateInboundKeepsKeyChangesTag(t *testing.T) {
	s, _ := newTestService(t)
	in, _ := s.CreateInbound("vless-reality", "v1", 8443, map[string]any{"handshake": "a.com"})
	d, _ := Get("vless-reality")
	before, _ := d.PublicInfo(in.Settings)
	up, err := s.UpdateInbound(in.ID, "v1b", 9443, map[string]any{"handshake": "b.com"})
	if err != nil {
		t.Fatalf("UpdateInbound: %v", err)
	}
	after, _ := d.PublicInfo(up.Settings)
	if up.Tag != "v1b" || up.Port != 9443 {
		t.Fatalf("not updated: %+v", up)
	}
	if after["realityPublicKey"] != before["realityPublicKey"] {
		t.Fatal("UpdateInbound must keep the key")
	}
	if after["serverName"] != "b.com" {
		t.Fatalf("serverName=%v", after["serverName"])
	}
}

func TestUpdateInboundDuplicateTag(t *testing.T) {
	s, _ := newTestService(t)
	_, _ = s.CreateInbound("vless-reality", "v1", 8443, nil)
	in2, _ := s.CreateInbound("vless-reality", "v2", 8444, nil)
	if _, err := s.UpdateInbound(in2.ID, "v1", 8444, nil); err != ErrTagExists {
		t.Fatalf("err=%v, want ErrTagExists", err)
	}
	if _, err := s.UpdateInbound(in2.ID, "v2", 8444, nil); err != nil {
		t.Fatalf("same-tag update should pass: %v", err)
	}
}

func TestResetInboundKeysChangesKey(t *testing.T) {
	s, _ := newTestService(t)
	in, _ := s.CreateInbound("vless-reality", "v1", 8443, nil)
	d, _ := Get("vless-reality")
	before, _ := d.PublicInfo(in.Settings)
	r, _ := s.ResetInboundKeys(in.ID)
	after, _ := d.PublicInfo(r.Settings)
	if after["realityPublicKey"] == before["realityPublicKey"] {
		t.Fatal("reset must change the key")
	}
}
