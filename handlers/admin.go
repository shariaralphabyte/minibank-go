package handlers

import (
    "encoding/json"
    "net/http"
    "strconv"
    "time"

    "minibank-go/middleware"
    "minibank-go/models"
    "minibank-go/utils"
)

func (h *Handlers) GetPendingKYC(w http.ResponseWriter, r *http.Request) {
    var kycRecords []models.KYC
    if err := h.db.Where("status = ?", "pending").
        Preload("User").
        Find(&kycRecords).Error; err != nil {
        sendError(w, http.StatusInternalServerError, "Failed to fetch pending KYC records", err.Error())
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(kycRecords)
}

func (h *Handlers) VerifyKYC(w http.ResponseWriter, r *http.Request) {
    claims := middleware.GetUserFromContext(r)
    if claims == nil {
        sendError(w, http.StatusUnauthorized, "Invalid or missing token", nil)
        return
    }

    var req models.KYCVerificationRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        sendError(w, http.StatusBadRequest, "Invalid request body", err.Error())
        return
    }

    if err := utils.ValidateStruct(req); err != nil {
        errors := utils.FormatValidationError(err)
        sendError(w, http.StatusBadRequest, "Validation failed", errors)
        return
    }

    // Begin transaction
    tx := h.db.Begin()
    defer func() {
        if r := recover(); r != nil {
            tx.Rollback()
        }
    }()

    // Validate KYC status
    if req.Status != "verified" && req.Status != "rejected" {
        tx.Rollback()
        sendError(w, http.StatusBadRequest, "Invalid KYC status", "Status must be either 'verified' or 'rejected'")
        return
    }

    // Validate rejection reason for rejected status
    if req.Status == "rejected" && req.RejectionReason == "" {
        tx.Rollback()
        sendError(w, http.StatusBadRequest, "Rejection reason is required", "Please provide a reason for rejection")
        return
    }

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
        sendError(w, http.StatusInternalServerError, "Failed to update KYC record", err.Error())
        return
    }

    // Update user KYC status
    var kyc models.KYC
    if err := tx.First(&kyc, req.KYCID).Error; err != nil {
        tx.Rollback()
        sendError(w, http.StatusNotFound, "KYC record not found", err.Error())
        return
    }

    if err := tx.Model(&models.User{}).Where("id = ?", kyc.UserID).Update("kyc_status", req.Status).Error; err != nil {
        tx.Rollback()
        sendError(w, http.StatusInternalServerError, "Failed to update user KYC status", err.Error())
        return
    }

    if err := tx.Commit().Error; err != nil {
        sendError(w, http.StatusInternalServerError, "Failed to commit transaction", err.Error())
        return
    }

    h.logAudit(&claims.UserID, "UPDATE", "KYC", 
        "KYC verification: "+req.Status, r.RemoteAddr, r.UserAgent())

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]interface{}{
        "message": "KYC verification updated successfully",
        "status":  req.Status,
        "kyc_id":  req.KYCID,
        "user_id": kyc.UserID,
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

    var total int64
    if err := h.db.Model(&models.AuditLog{}).Count(&total).Error; err != nil {
        sendError(w, http.StatusInternalServerError, "Failed to fetch audit logs", err.Error())
        return
    }

    var auditLogs []models.AuditLog
    if err := h.db.Preload("User").
        Order("created_at DESC").
        Limit(limit).
        Offset(offset).
        Find(&auditLogs).Error; err != nil {
        sendError(w, http.StatusInternalServerError, "Failed to fetch audit logs", err.Error())
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]interface{}{
        "audit_logs": auditLogs,
        "page":       page,
        "limit":      limit,
        "total":      total,
    })
}

func (h *Handlers) GetAllUsers(w http.ResponseWriter, r *http.Request) {
	emailQuery := r.URL.Query().Get("email")

	if emailQuery != "" {
		var user models.User
		if err := h.db.Where("email = ?", emailQuery).First(&user).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				sendError(w, http.StatusNotFound, "User not found with specified email", nil)
			} else {
				sendError(w, http.StatusInternalServerError, "Database error fetching user by email", map[string]string{"original_error": err.Error()})
			}
			return
		}
		user.Password = "" // Clear password
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"user": user})
		return
	}

	// Existing pagination logic if no email query parameter is provided
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page <= 0 {
		page = 1
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	offset := (page - 1) * limit

	var total int64
	if err := h.db.Model(&models.User{}).Count(&total).Error; err != nil {
		sendError(w, http.StatusInternalServerError, "Failed to count users", err.Error())
		return
	}

	var users []models.User
	// The Select statement already omits the password, which is good.
	if err := h.db.Select("id, email, phone, first_name, last_name, balance, is_active, kyc_status, created_at, updated_at, is_admin").
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&users).Error; err != nil {
		sendError(w, http.StatusInternalServerError, "Failed to fetch users", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"users": users,
		"page":  page,
		"limit": limit,
		"total": total,
	})
}