package main

import (
    "log"
    "net/http"

    "minibank-go/config"
    "minibank-go/database"
    "minibank-go/handlers"
    "minibank-go/middleware"
    "minibank-go/utils"

    "github.com/gorilla/mux"
    "github.com/joho/godotenv"
)

func main() {
    // Load environment variables
    if err := godotenv.Load(); err != nil {
        log.Println("No .env file found")
    }

    // Initialize config
    cfg := config.Load()
    
    // Validate configuration
    config.ValidateConfig(cfg)

    // Initialize encryption
    if err := utils.InitializeEncryption(cfg.EncryptionKey); err != nil {
        log.Fatal("Failed to initialize encryption:", err)
    }

    // Initialize JWT
    if err := utils.InitializeJWT(cfg.JWTSecret); err != nil {
        log.Fatal("Failed to initialize JWT:", err)
    }

    // Initialize database
    db, err := database.Initialize(cfg.DatabaseURL)
    if err != nil {
        log.Fatal("Failed to initialize database:", err)
    }

    // Initialize handlers with config
    h := handlers.NewHandlers(db, cfg)

    // Initialize router
    r := mux.NewRouter()

    // Apply global middleware
    r.Use(middleware.CORS)
    r.Use(middleware.RateLimit)

    // Public routes
    r.HandleFunc("/api/register", h.Register).Methods("POST")
    r.HandleFunc("/api/login", h.Login).Methods("POST")
    r.HandleFunc("/api/health", h.HealthCheck).Methods("GET")

    // Protected routes
    protected := r.PathPrefix("/api").Subrouter()
    protected.Use(middleware.JWTAuth)

    // Debug endpoint (protected but not admin-only)
    protected.HandleFunc("/debug/token", h.DebugToken).Methods("GET")

    // User routes
    protected.HandleFunc("/user/profile", h.GetProfile).Methods("GET")
    protected.HandleFunc("/user/profile", h.UpdateProfile).Methods("PUT")

    // KYC routes
    protected.HandleFunc("/kyc/submit", h.SubmitKYC).Methods("POST")
    protected.HandleFunc("/kyc/status", h.GetKYCStatus).Methods("GET")

    // Transaction routes
    protected.HandleFunc("/transactions", h.GetTransactions).Methods("GET")
    protected.HandleFunc("/transactions/deposit", h.Deposit).Methods("POST")
    protected.HandleFunc("/transactions/withdraw", h.Withdraw).Methods("POST")
    protected.HandleFunc("/transactions/transfer", h.Transfer).Methods("POST")

    // Admin routes
    adminRoutes := protected.PathPrefix("/admin").Subrouter()
    adminRoutes.Use(middleware.AdminAuth)
    adminRoutes.HandleFunc("/kyc/pending", h.GetPendingKYC).Methods("GET")
    adminRoutes.HandleFunc("/kyc/verify", h.VerifyKYC).Methods("POST")
    adminRoutes.HandleFunc("/audit-logs", h.GetAuditLogs).Methods("GET")
    adminRoutes.HandleFunc("/users", h.GetAllUsers).Methods("GET")

    port := cfg.Port
    if port == "" {
        port = "8080"
    }

    log.Printf("Server starting on port %s", port)
    log.Printf("Environment: %s", cfg.Environment)
    log.Printf("Database: %s", cfg.DatabaseURL)
    if cfg.Environment == "development" {
        log.Printf("Admin Code: %s", cfg.AdminCode)
        log.Printf("Debug endpoint available at: /api/debug/token")
    }
    log.Fatal(http.ListenAndServe(":"+port, r))
}