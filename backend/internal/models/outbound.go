package models

import "time"

// Outbound is an upstream proxy (http or socks5) that user traffic can egress
// through. Username/Password are the upstream credentials (optional).
type Outbound struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Tag       string    `gorm:"uniqueIndex;not null" json:"tag"`
	Type      string    `gorm:"not null" json:"type"` // "http" | "socks5"
	Server    string    `gorm:"not null" json:"server"`
	Port      uint16    `gorm:"not null" json:"port"`
	Username  string    `json:"username"`
	Password  string    `json:"password"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}
