package database

import (
    "minibank-go/models"

    "gorm.io/driver/sqlite"
    "gorm.io/gorm"
    "gorm.io/gorm/logger"
)

func Initialize(databaseURL string) (*gorm.DB, error) {
    db, err := gorm.Open(sqlite.Open(databaseURL), &gorm.Config{
        Logger: logger.Default.LogMode(logger.Info),
    })
    if err != nil {
        return nil, err
    }

    // Auto-migrate models
    err = db.AutoMigrate(
        &models.User{},
        &models.KYC{},
        &models.Transaction{},
        &models.AuditLog{},
    )
    if err != nil {
        return nil, err
    }

    return db, nil
}