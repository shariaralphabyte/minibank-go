package handlers

import (
    "encoding/json"
    "fmt"
    "net/http"
    "time"

    "gorm.io/gorm"

    "github.com/google/uuid"

    "minibank-go/config"
    "minibank-go/models"
)

type Handlers struct {
    db     *gorm.DB
    config *config.Config
}

func NewHandlers(db *gorm.DB, cfg *config.Config) *Handlers {
    return &Handlers{
        db:     db,
        config: cfg,
    }
}

func (h *Handlers) HealthCheck(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]interface{}{
        "status":    "healthy",
        "timestamp": time.Now(),
        "service":   "MiniBankGo",
        "version":   "1.0.0",
    })
}

func (h *Handlers) logAudit(userID *uint, action, resource, details, ipAddress, userAgent string) {
    audit := models.AuditLog{
        UserID:    userID,
        Action:    action,
        Resource:  resource,
        Details:   details,
        IPAddress: ipAddress,
        UserAgent: userAgent,
    }
    h.db.Create(&audit)
}

func (h *Handlers) generateReference() string {
    return fmt.Sprintf("TXN_%s", uuid.New().String())
}