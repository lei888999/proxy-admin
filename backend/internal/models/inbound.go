package models

import "time"

type Inbound struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Tag       string    `gorm:"uniqueIndex;not null" json:"tag"`
	Type      string    `gorm:"not null" json:"type"`
	Network   string    `gorm:"not null" json:"network"`
	Port      uint16    `gorm:"not null" json:"port"`
	Settings  string    `gorm:"not null" json:"-"`
	Users     []User    `gorm:"constraint:OnDelete:CASCADE" json:"-"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type User struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	InboundID  uint      `gorm:"index;not null" json:"inboundId"`
	Name       string    `gorm:"not null" json:"name"`
	Credential string    `gorm:"not null" json:"credential"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}
