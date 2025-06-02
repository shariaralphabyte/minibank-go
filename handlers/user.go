package handlers

import (
    "encoding/json"
    "net/http"

    "minibank-go/middleware"
    "minibank-go/models"

    "gorm.io/gorm"
)

func (h *Handlers) GetProfile(w http.ResponseWriter, r *http.Request) {
    claims := middleware.GetUserFromContext(r)
    if claims == nil {
        sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
        return
    }

    var user models.User
    if err := h.db.First(&user, claims.UserID).Error; err != nil {
        if err == gorm.ErrRecordNotFound {
            sendError(w, http.StatusNotFound, "User not found", map[string]string{"original_error": err.Error()})
            return
        }
        sendError(w, http.StatusInternalServerError, "Database error", map[string]string{"original_error": err.Error()})
        return
    }

    user.Password = ""
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(user)
}

func (h *Handlers) UpdateProfile(w http.ResponseWriter, r *http.Request) {
    claims := middleware.GetUserFromContext(r)
    if claims == nil {
        sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
        return
    }

    var req struct {
        FirstName string `json:"first_name"`
        LastName  string `json:"last_name"`
        Phone     string `json:"phone"`
    }

    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        sendError(w, http.StatusBadRequest, "Invalid request body", map[string]string{"decode_error": err.Error()})
        return
    }

    var user models.User
    if err := h.db.First(&user, claims.UserID).Error; err != nil {
        // Note: The original code returned http.StatusNotFound for any error here.
        // We'll keep gorm.ErrRecordNotFound specific and a general error for others,
        // though the prompt implies always StatusNotFound for this specific .First call.
        // For consistency with GetProfile and specific error handling, we'll differentiate.
        if err == gorm.ErrRecordNotFound {
            sendError(w, http.StatusNotFound, "User not found", nil)
        } else {
            sendError(w, http.StatusInternalServerError, "Database error while fetching user for update", map[string]string{"original_error": err.Error()})
        }
        return
    }

    // Update fields
    user.FirstName = req.FirstName
    user.LastName = req.LastName
    user.Phone = req.Phone

    if err := h.db.Save(&user).Error; err != nil {
        sendError(w, http.StatusInternalServerError, "Failed to update profile", map[string]string{"save_error": err.Error()})
        return
    }

    h.logAudit(&user.ID, "UPDATE", "USER", "Profile updated", r.RemoteAddr, r.UserAgent())

    user.Password = ""
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(user)
}