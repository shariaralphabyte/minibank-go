package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"minibank-go/middleware"
	"minibank-go/models"
	"minibank-go/utils"

	"gorm.io/gorm"
)

func (h *Handlers) SubmitKYC(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserFromContext(r)
	if claims == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req models.KYCRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := utils.ValidateStruct(req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":   "Validation failed",
			"details": utils.FormatValidationError(err),
		})
		return
	}

	// Validate PAN format
	if !utils.ValidatePAN(req.PAN) {
		http.Error(w, "Invalid PAN format", http.StatusBadRequest)
		return
	}

	// Validate Aadhaar if provided
	if req.AadhaarNumber != "" && !utils.ValidateAadhaar(req.AadhaarNumber) {
		http.Error(w, "Invalid Aadhaar format", http.StatusBadRequest)
		return
	}

	// Check age validation
	if !utils.IsValidAge(req.DateOfBirth) {
		http.Error(w, "Must be at least 18 years old", http.StatusBadRequest)
		return
	}

	// Check if KYC already exists
	var existingKYC models.KYC
	if err := h.db.Where("user_id = ?", claims.UserID).First(&existingKYC).Error; err == nil {
		http.Error(w, "KYC already submitted", http.StatusConflict)
		return
	}

	// Encrypt sensitive data
	encryptedPAN, err := utils.EncryptSensitiveData(req.PAN)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":   "Failed to encrypt PAN data",
			"details": fmt.Sprintf("Encryption error: %v", err),
		})
		return
	}

	var encryptedAadhaar string
	if req.AadhaarNumber != "" {
		encryptedAadhaar, err = utils.EncryptSensitiveData(req.AadhaarNumber)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error":   "Failed to encrypt Aadhaar data",
				"details": fmt.Sprintf("Encryption error: %v", err),
			})
			return
		}
	}

	// Create KYC record
	kyc := models.KYC{
		UserID:         claims.UserID,
		PAN:            encryptedPAN,
		AadhaarNumber:  encryptedAadhaar,
		PassportNumber: req.PassportNumber,
		DateOfBirth:    req.DateOfBirth,
		Address:        req.Address,
		City:           req.City,
		State:          req.State,
		PinCode:        req.PinCode,
		Status:         "pending",
	}

	if err := h.db.Create(&kyc).Error; err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":   "Failed to submit KYC",
			"details": fmt.Sprintf("Database error: %v", err),
		})
		return
	}

	h.logAudit(&claims.UserID, "CREATE", "KYC", "KYC submitted", r.RemoteAddr, r.UserAgent())

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "KYC submitted successfully",
		"kyc_id":  kyc.ID,
		"status":  "pending",
	})
}

func (h *Handlers) GetKYCStatus(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserFromContext(r)
	if claims == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var kyc models.KYC
	if err := h.db.Where("user_id = ?", claims.UserID).First(&kyc).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status": "not_submitted",
			})
			return
		}
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"status":           kyc.Status,
		"submission_date":  kyc.CreatedAt,
		"rejection_reason": kyc.RejectionReason,
	}

	if kyc.VerifiedAt != nil {
		response["verified_at"] = kyc.VerifiedAt
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
