package models

import "time"

type Admin struct {
	ID           uint   `gorm:"primaryKey"`
	Username     string `gorm:"uniqueIndex;not null"`
	PasswordHash string `gorm:"not null"`
	// TokenVersion is stamped into every session cookie and bumped on a password
	// change, so changing a leaked password actually revokes the 7-day sessions
	// issued under it instead of leaving them valid.
	TokenVersion int `gorm:"not null;default:1"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
