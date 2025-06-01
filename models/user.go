package models

import (
    "time"

    "gorm.io/gorm"
)

type User struct {
    ID          uint           `json:"id" gorm:"primaryKey"`
    Email       string         `json:"email" gorm:"uniqueIndex;not null"`
    Phone       string         `json:"phone" gorm:"uniqueIndex;not null"`
    Password    string         `json:"-" gorm:"not null"`
    FirstName   string         `json:"first_name" gorm:"not null"`
    LastName    string         `json:"last_name" gorm:"not null"`
    Balance     float64        `json:"balance" gorm:"default:0"`
    IsActive    bool           `json:"is_active" gorm:"default:true"`
    IsAdmin     bool           `json:"is_admin" gorm:"default:false"`
    KYCStatus   string         `json:"kyc_status" gorm:"default:pending"` // pending, verified, rejected
    Verified    bool           `json:"verified" gorm:"default:false"`
    CreatedAt   time.Time      `json:"created_at"`
    UpdatedAt   time.Time      `json:"updated_at"`
    DeletedAt   gorm.DeletedAt `json:"-" gorm:"index"`
}

type RegisterRequest struct {
    Email     string `json:"email" validate:"required,email"`
    Phone     string `json:"phone" validate:"required,min=10,max=15"`
    Password  string `json:"password" validate:"required,min=8"`
    FirstName string `json:"first_name" validate:"required,min=2"`
    LastName  string `json:"last_name" validate:"required,min=2"`
    AdminCode string `json:"admin_code,omitempty"` // Optional field for admin registration
}

type LoginRequest struct {
    Email    string `json:"email" validate:"required,email"`
    Password string `json:"password" validate:"required"`
}

type LoginResponse struct {
    Token string `json:"token"`
    User  User   `json:"user"`
}