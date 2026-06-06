package models

import "time"

type Inbound struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Tag       string    `gorm:"uniqueIndex;not null" json:"tag"`
	Type      string    `gorm:"not null" json:"type"`
	Network   string    `gorm:"not null" json:"network"`
	Port      uint16    `gorm:"not null" json:"port"`
	Settings  string    `gorm:"not null" json:"-"`
	Users     []User    `gorm:"many2many:user_inbounds;" json:"-"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type User struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"not null" json:"name"`
	UUID      string    `gorm:"not null" json:"uuid"`
	Password  string    `gorm:"not null" json:"password"`
	SubToken  string    `gorm:"uniqueIndex" json:"subToken"`
	UpBytes   int64     `gorm:"not null;default:0" json:"upBytes"`
	DownBytes  int64     `gorm:"not null;default:0" json:"downBytes"`
	OutboundID *uint    `json:"outboundId"`
	Inbounds  []Inbound `gorm:"many2many:user_inbounds;" json:"-"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}
