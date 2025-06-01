package config

import (
    "log"
    "os"
)

type TransactionLimits struct {
    DailyDepositLimit  float64
    DailyWithdrawLimit float64
    DailyTransferLimit float64
}

type AMLRules struct {
    MonthlyThreshold         float64
    DailyTransactionLimit   int
}

type Config struct {
    DatabaseURL        string
    JWTSecret          string
    EncryptionKey      string
    AdminCode          string
    Port               string
    Environment        string
    TransactionLimits  TransactionLimits
    AMLRules           AMLRules
    MaxTransferAmount  float64
    DailyTransferLimit float64
}

func Load() *Config {
    return &Config{
        DatabaseURL:        getEnv("DATABASE_URL", "minibank.db"),
        JWTSecret:          getEnv("JWT_SECRET", "your-secret-key-change-in-production"),
        EncryptionKey:      getEnv("ENCRYPTION_KEY", "MiniBankGo2025SecureKey123456789"),
        AdminCode:          getEnv("ADMIN_CODE", "MINIBANK_ADMIN_2025"),
        Port:               getEnv("PORT", "8080"),
        Environment:        getEnv("ENVIRONMENT", "development"),
        TransactionLimits: TransactionLimits{
            DailyDepositLimit:  10000.0,
            DailyWithdrawLimit: 5000.0,
            DailyTransferLimit: 50000.0,
        },
        AMLRules: AMLRules{
            MonthlyThreshold:        100000.0,
            DailyTransactionLimit:   10,
        },
        MaxTransferAmount:  10000.0,
        DailyTransferLimit: 50000.0,
    }
}

func getEnv(key, defaultValue string) string {
    if value := os.Getenv(key); value != "" {
        return value
    }
    return defaultValue
}

func ValidateConfig(cfg *Config) {
    if len(cfg.EncryptionKey) != 32 {
        log.Fatalf("ENCRYPTION_KEY must be exactly 32 characters, got %d", len(cfg.EncryptionKey))
    }
    if len(cfg.JWTSecret) < 32 {
        log.Printf("WARNING: JWT_SECRET should be at least 32 characters for security")
    }
    if cfg.Environment == "production" && cfg.AdminCode == "MINIBANK_ADMIN_2025" {
        log.Printf("WARNING: Change ADMIN_CODE in production environment")
    }
}