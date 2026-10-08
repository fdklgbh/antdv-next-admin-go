package model

import "time"

type User struct {
	ID           int64  `gorm:"primaryKey;autoIncrement"`
	Username     string `gorm:"size:64;not null;uniqueIndex"`
	PasswordHash string `gorm:"size:255;not null"`
	Phone        string `gorm:"size:32;not null;default:''"`
	Gender       string `gorm:"size:16;not null;default:''"`
	Bio          string `gorm:"size:200;not null;default:''"`
	Email        string `gorm:"size:254;not null;default:''"`
	DisplayName  string `gorm:"size:128;not null"`
	Enabled      bool   `gorm:"not null"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (User) TableName() string { return "auth_users" }

type RefreshSession struct {
	Principal Principal `json:"principal"`
	SessionID string    `json:"sessionId"`
	ExpiresAt time.Time `json:"expiresAt"`
	Remember  bool      `json:"remember"`
}
