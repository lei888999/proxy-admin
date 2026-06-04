package database

import (
	"testing"

	"singbox-admin/internal/models"
	"golang.org/x/crypto/bcrypt"
)

func TestInitCreatesDefaultAdmin(t *testing.T) {
	db, err := Init(":memory:", "admin", "mnice7082")
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	var a models.Admin
	if err := db.Where("username = ?", "admin").First(&a).Error; err != nil {
		t.Fatalf("default admin not found: %v", err)
	}
	if bcrypt.CompareHashAndPassword([]byte(a.PasswordHash), []byte("mnice7082")) != nil {
		t.Fatal("default admin password hash mismatch")
	}
}

func TestInitIdempotent(t *testing.T) {
	db, _ := Init(":memory:", "admin", "mnice7082")
	if err := seedDefaultAdmin(db, "admin", "mnice7082"); err != nil {
		t.Fatalf("seed: %v", err)
	}
	var count int64
	db.Model(&models.Admin{}).Count(&count)
	if count != 1 {
		t.Fatalf("admin count = %d, want 1", count)
	}
}
