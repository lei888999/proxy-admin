package models

// Meta is a small panel-owned key/value store (e.g. generated API addresses
// and secrets that must stay stable across restarts).
type Meta struct {
	Key   string `gorm:"primaryKey"`
	Value string `gorm:"not null"`
}
