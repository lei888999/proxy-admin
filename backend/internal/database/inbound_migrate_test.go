package database

import (
	"testing"

	"singbox-admin/internal/models"
)

func TestInitMigratesInboundAndUser(t *testing.T) {
	db, err := Init(":memory:", "admin", "mnice7082")
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	if !db.Migrator().HasTable(&models.Inbound{}) {
		t.Fatal("inbounds table missing")
	}
	if !db.Migrator().HasTable(&models.User{}) {
		t.Fatal("users table missing")
	}
}
