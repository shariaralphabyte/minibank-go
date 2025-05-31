package handlers

import (
    "encoding/json"
    "net/http"
    "strconv"
    "time"

    "minibank-go/middleware"
    "minibank-go/models"

)

func (h *Handlers) GetPendingKYC(w http.ResponseWriter, r *http.Request) {
    var kycRecords []models.KYC
    if err := h.db.Where("status = ?", "pending").
        Preload("User").
        Find(&kycRecords).Error; err != nil {
        http.Error(w, "Failed to fetch pending KYC records", http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(kycRecords)
}

func (h *Handlers) VerifyKYC(w http.ResponseWriter, r *http.Request) {
    claims := middleware.GetUserFromContext(r)
    if claims == nil {
        http.Error(w, "Unauthorized", http.StatusUnauthorized)
        return
    }

    var req models.KYCVerificationRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid request body", http.StatusBadRequest)
        return
    }

    // Begin transaction
    tx := h.db.Begin()
    defer func() {
        if r := recover(); r != nil {
            tx.Rollback()
        }
    }()

    // Update KYC record
    now := time.Now()
    updateData := map[string]interface{}{
        "status":       req.Status,
        "verified_by":  claims.UserID,
        "verified_at":  &now,
    }

    if req.Status == "rejected" {
        updateData["rejection_reason"] = req.RejectionReason
    }

    if err := tx.Model(&models.KYC{}).Where("id = ?", req.KYCID).Updates(updateData).Error; err != nil {
        tx.Rollback()
        http.Error(w, "Failed to update KYC record", http.StatusInternalServerError)
        return
    }

    // Update user KYC status
    var kyc models.KYC
    if err := tx.First(&kyc, req.KYCID).Error; err != nil {
        tx.Rollback()
        http.Error(w, "KYC record not found", http.StatusNotFound)
        return
    }

    if err := tx.Model(&models.User{}).Where("id = ?", kyc.UserID).Update("kyc_status", req.Status).Error; err != nil {
        tx.Rollback()
        http.Error(w, "Failed to update user KYC status", http.StatusInternalServerError)
        return
    }

    if err := tx.Commit().Error; err != nil {
        http.Error(w, "Failed to commit transaction", http.StatusInternalServerError)
        return
    }

    h.logAudit(&claims.UserID, "UPDATE", "KYC", 
        "KYC verification: "+req.Status, r.RemoteAddr, r.UserAgent())

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]string{
        "message": "KYC verification updated successfully",
    })
}

func (h *Handlers) GetAuditLogs(w http.ResponseWriter, r *http.Request) {
    page, _ := strconv.Atoi(r.URL.Query().Get("page"))
    if page <= 0 {
        page = 1
    }
    limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
    if limit <= 0 || limit > 100 {
        limit = 50
    }
    offset := (page - 1) * limit

    var auditLogs []models.AuditLog
    if err := h.db.Preload("User").
        Order("created_at DESC").
        Limit(limit).
        Offset(offset).
        Find(&auditLogs).Error; err != nil {
        http.Error(w, "Failed to fetch audit logs", http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(auditLogs)
}

func (h *Handlers) GetAllUsers(w http.ResponseWriter, r *http.Request) {
    page, _ := strconv.Atoi(r.URL.Query().Get("page"))
    if page <= 0 {
        page = 1
    }
    limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
    if limit <= 0 || limit > 100 {
        limit = 20
    }
    offset := (page - 1) * limit

    var users []models.User
    if err := h.db.Select("id, email, phone, first_name, last_name, balance, is_active, kyc_status, created_at, updated_at").
        Order("created_at DESC").
        Limit(limit).
        Offset(offset).
        Find(&users).Error; err != nil {
        http.Error(w, "Failed to fetch users", http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(users)
}