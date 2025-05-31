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
        http.Error(w, "Unauthorized", http.StatusUnauthorized)
        return
    }

    var user models.User
    if err := h.db.First(&user, claims.UserID).Error; err != nil {
        if err == gorm.ErrRecordNotFound {
            http.Error(w, "User not found", http.StatusNotFound)
            return
        }
        http.Error(w, "Database error", http.StatusInternalServerError)
        return
    }

    user.Password = ""
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(user)
}

func (h *Handlers) UpdateProfile(w http.ResponseWriter, r *http.Request) {
    claims := middleware.GetUserFromContext(r)
    if claims == nil {
        http.Error(w, "Unauthorized", http.StatusUnauthorized)
        return
    }

    var req struct {
        FirstName string `json:"first_name"`
        LastName  string `json:"last_name"`
        Phone     string `json:"phone"`
    }

    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid request body", http.StatusBadRequest)
        return
    }

    var user models.User
    if err := h.db.First(&user, claims.UserID).Error; err != nil {
        http.Error(w, "User not found", http.StatusNotFound)
        return
    }

    // Update fields
    user.FirstName = req.FirstName
    user.LastName = req.LastName
    user.Phone = req.Phone

    if err := h.db.Save(&user).Error; err != nil {
        http.Error(w, "Failed to update profile", http.StatusInternalServerError)
        return
    }

    h.logAudit(&user.ID, "UPDATE", "USER", "Profile updated", r.RemoteAddr, r.UserAgent())

    user.Password = ""
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(user)
}