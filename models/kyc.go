package models

import (
    "time"

    "gorm.io/gorm"
)

type KYC struct {
    ID             uint           `json:"id" gorm:"primaryKey"`
    UserID         uint           `json:"user_id" gorm:"not null"`
    User           User           `json:"user" gorm:"foreignKey:UserID"`
    PAN            string         `json:"pan" gorm:"not null"`
    AadhaarNumber  string         `json:"aadhaar_number"`
    PassportNumber string         `json:"passport_number"`
    DateOfBirth    time.Time      `json:"date_of_birth" gorm:"not null"`
    Address        string         `json:"address" gorm:"not null"`
    City           string         `json:"city" gorm:"not null"`
    State          string         `json:"state" gorm:"not null"`
    PinCode        string         `json:"pin_code" gorm:"not null"`
    Status         string         `json:"status" gorm:"default:pending"` // pending, verified, rejected
    RejectionReason string        `json:"rejection_reason"`
    VerifiedBy     uint           `json:"verified_by"`
    VerifiedAt     *time.Time     `json:"verified_at"`
    CreatedAt      time.Time      `json:"created_at"`
    UpdatedAt      time.Time      `json:"updated_at"`
    DeletedAt      gorm.DeletedAt `json:"-" gorm:"index"`
}

type KYCRequest struct {
    PAN            string    `json:"pan" validate:"required,len=10"`
    AadhaarNumber  string    `json:"aadhaar_number" validate:"omitempty,len=12"`
    PassportNumber string    `json:"passport_number" validate:"omitempty,min=8"`
    DateOfBirth    time.Time `json:"date_of_birth" validate:"required"`
    Address        string    `json:"address" validate:"required,min=10"`
    City           string    `json:"city" validate:"required,min=2"`
    State          string    `json:"state" validate:"required,min=2"`
    PinCode        string    `json:"pin_code" validate:"required,len=6"`
}

type KYCVerificationRequest struct {
    KYCID           uint   `json:"kyc_id" validate:"required"`
    Status          string `json:"status" validate:"required,oneof=verified rejected"`
    RejectionReason string `json:"rejection_reason"`
}