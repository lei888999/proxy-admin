package inbound

import (
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"singbox-admin/internal/models"
)

func TestUserSubscription(t *testing.T) {
	db, _ := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	db.AutoMigrate(&models.Inbound{}, &models.User{}, &models.Meta{}, &models.Outbound{})
	s := NewService(db, &fakeWriter{})
	if _, err := s.CreateInbound("vless-reality", "v1", 8443, nil); err != nil {
		t.Fatalf("inbound: %v", err)
	}
	var in models.Inbound
	db.First(&in, "tag = ?", "v1")
	u, _ := s.CreateUser("alice", []uint{in.ID}, nil)

	yaml, err := s.UserSubscription(u.SubToken, "vps.example.com")
	if err != nil {
		t.Fatalf("UserSubscription: %v", err)
	}
	if !strings.Contains(yaml, "proxies:") || !strings.Contains(yaml, "vps.example.com") {
		t.Fatalf("subscription missing proxies/server:\n%s", yaml)
	}
	if !strings.Contains(yaml, u.UUID) {
		t.Fatal("subscription should contain the user's uuid")
	}
	if !strings.Contains(yaml, "节点") {
		t.Fatal("subscription should contain the proxy group")
	}

	if _, err := s.UserSubscription("nope", "vps.example.com"); err != ErrNotFound {
		t.Fatalf("unknown token should be ErrNotFound, got %v", err)
	}
}
