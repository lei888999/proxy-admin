package database

import (
	"log"
	"os"
	"strings"

	"github.com/glebarez/sqlite"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"singbox-admin/internal/models"
)

func Init(dbPath, defaultUser, defaultPass string) (*gorm.DB, error) {
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	restrictFilePerms(dbPath)
	if err := db.AutoMigrate(&models.Admin{}, &models.Inbound{}, &models.User{}, &models.Meta{}, &models.Outbound{}); err != nil {
		return nil, err
	}
	if err := seedDefaultAdmin(db, defaultUser, defaultPass); err != nil {
		return nil, err
	}
	return db, nil
}

// restrictFilePerms takes the database file down to owner-only. It holds every
// user's UUID and password, each upstream proxy's credentials, the Reality
// private key and the hysteria2 TLS key in cleartext, so a 0644 default would
// hand all of it to any other account on the host.
func restrictFilePerms(dbPath string) {
	if dbPath == "" || strings.Contains(dbPath, ":memory:") {
		return
	}
	if err := os.Chmod(dbPath, 0600); err != nil && !os.IsNotExist(err) {
		log.Printf("WARN: could not restrict permissions on %s: %v", dbPath, err)
	}
}

func seedDefaultAdmin(db *gorm.DB, user, pass string) error {
	var count int64
	if err := db.Model(&models.Admin{}).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(pass), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return db.Create(&models.Admin{Username: user, PasswordHash: string(hash)}).Error
}
