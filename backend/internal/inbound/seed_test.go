package inbound

import (
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"singbox-admin/internal/models"
)

func TestSeedDefaultsCreatesTwo(t *testing.T) {
	db, _ := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	db.AutoMigrate(&models.Inbound{}, &models.User{}, &models.Meta{})
	s := NewService(db, &fakeWriter{})
	if err := SeedDefaults(s); err != nil {
		t.Fatalf("SeedDefaults: %v", err)
	}
	var n int64
	db.Model(&models.Inbound{}).Count(&n)
	if n != 2 {
		t.Fatalf("seeded %d, want 2", n)
	}
	if err := SeedDefaults(s); err != nil {
		t.Fatal(err)
	}
	db.Model(&models.Inbound{}).Count(&n)
	if n != 2 {
		t.Fatalf("after re-seed %d, want 2", n)
	}
}
