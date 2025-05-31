package models

import (
    "time"

    "gorm.io/gorm"
)

type AuditLog struct {
    ID        uint           `json:"id" gorm:"primaryKey"`
    UserID    *uint          `json:"user_id"`
    User      *User          `json:"user" gorm:"foreignKey:UserID"`
    Action    string         `json:"action" gorm:"not null"`
    Resource  string         `json:"resource" gorm:"not null"`
    Details   string         `json:"details"`
    IPAddress string         `json:"ip_address"`
    UserAgent string         `json:"user_agent"`
    CreatedAt time.Time      `json:"created_at"`
    UpdatedAt time.Time      `json:"updated_at"`
    DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
}