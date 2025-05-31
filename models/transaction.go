package models

import (
    "time"

    "gorm.io/gorm"
)

type Transaction struct {
    ID            uint           `json:"id" gorm:"primaryKey"`
    UserID        uint           `json:"user_id" gorm:"not null"`
    User          User           `json:"user" gorm:"foreignKey:UserID"`
    Type          string         `json:"type" gorm:"not null"` // deposit, withdraw, transfer_out, transfer_in
    Amount        float64        `json:"amount" gorm:"not null"`
    BalanceBefore float64        `json:"balance_before" gorm:"not null"`
    BalanceAfter  float64        `json:"balance_after" gorm:"not null"`
    ToUserID      *uint          `json:"to_user_id"`
    ToUser        *User          `json:"to_user" gorm:"foreignKey:ToUserID"`
    FromUserID    *uint          `json:"from_user_id"`
    FromUser      *User          `json:"from_user" gorm:"foreignKey:FromUserID"`
    Description   string         `json:"description"`
    Reference     string         `json:"reference"`
    Status        string         `json:"status" gorm:"default:completed"` // pending, completed, failed
    IPAddress     string         `json:"ip_address"`
    CreatedAt     time.Time      `json:"created_at"`
    UpdatedAt     time.Time      `json:"updated_at"`
    DeletedAt     gorm.DeletedAt `json:"-" gorm:"index"`
}

type TransactionRequest struct {
    Amount      float64 `json:"amount" validate:"required,min=1"`
    Description string  `json:"description"`
}

type TransferRequest struct {
    ToUserEmail string  `json:"to_user_email" validate:"required,email"`
    Amount      float64 `json:"amount" validate:"required,min=1"`
    Description string  `json:"description"`
}