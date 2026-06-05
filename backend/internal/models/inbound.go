package models

import "time"

type Inbound struct {
	ID                uint      `gorm:"primaryKey" json:"id"`
	Tag               string    `gorm:"uniqueIndex;not null" json:"tag"`
	Port              uint16    `gorm:"not null" json:"port"`
	Flow              string    `gorm:"not null" json:"flow"`
	RealityPrivateKey string    `gorm:"not null" json:"-"` // never exposed to the frontend
	RealityPublicKey  string    `gorm:"not null" json:"realityPublicKey"`
	RealityShortID    string    `gorm:"not null" json:"realityShortId"`
	Handshake         string    `gorm:"not null" json:"handshake"`
	HandshakePort     uint16    `gorm:"not null" json:"handshakePort"`
	ServerName        string    `gorm:"not null" json:"serverName"`
	Users             []User    `gorm:"constraint:OnDelete:CASCADE" json:"users"`
	CreatedAt         time.Time `json:"createdAt"`
	UpdatedAt         time.Time `json:"updatedAt"`
}

type User struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	InboundID uint      `gorm:"index;not null" json:"inboundId"`
	Name      string    `gorm:"not null" json:"name"`
	UUID      string    `gorm:"not null" json:"uuid"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}
