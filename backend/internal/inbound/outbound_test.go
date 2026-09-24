package inbound

import (
	"testing"

	"singbox-admin/internal/models"
)

func TestOutboundCRUD(t *testing.T) {
	s, _ := newTestService(t)
	o, err := s.CreateOutbound("socks5", "proxyA", "1.2.3.4", 1080, "u", "p")
	if err != nil {
		t.Fatalf("CreateOutbound: %v", err)
	}
	if o.ID == 0 {
		t.Fatal("no id")
	}
	if _, err := s.CreateOutbound("socks5", "proxyA", "5.6.7.8", 1080, "", ""); err != ErrTagExists {
		t.Fatalf("dup tag err=%v, want ErrTagExists", err)
	}
	if _, err := s.CreateOutbound("ftp", "x", "1.2.3.4", 1, "", ""); err != ErrInvalidType {
		t.Fatalf("bad type err=%v, want ErrInvalidType", err)
	}
	if _, err := s.CreateOutbound("http", "x", "", 8080, "", ""); err != ErrInvalidOutbound {
		t.Fatalf("empty server err=%v, want ErrInvalidOutbound", err)
	}
	if _, err := s.CreateOutbound("http", "x", "1.2.3.4", 0, "", ""); err != ErrInvalidOutbound {
		t.Fatalf("zero port err=%v, want ErrInvalidOutbound", err)
	}
	views, _ := s.ListOutboundViews()
	if len(views) != 1 || views[0].Tag != "proxyA" || views[0].Type != "socks5" {
		t.Fatalf("views=%v", views)
	}
	if _, err := s.UpdateOutbound(o.ID, "http", "proxyB", "9.9.9.9", 8080, "", ""); err != nil {
		t.Fatalf("UpdateOutbound: %v", err)
	}
}

func TestDeleteOutboundRejectsReferencedUsers(t *testing.T) {
	s, _ := newTestService(t)
	o, _ := s.CreateOutbound("http", "proxyA", "1.2.3.4", 8080, "", "")
	u, err := s.CreateUser("alice", nil, &o.ID)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if err := s.DeleteOutbound(o.ID); err != ErrOutboundInUse {
		t.Fatalf("DeleteOutbound err=%v, want ErrOutboundInUse", err)
	}
	var got models.User
	s.db.First(&got, u.ID)
	if got.OutboundID == nil || *got.OutboundID != o.ID {
		t.Fatalf("user outbound changed unexpectedly: %v", got.OutboundID)
	}
}
