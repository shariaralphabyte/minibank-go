package handlers

import (
    "encoding/json"
    "log"
    "net/http"
    "strings"

    "minibank-go/middleware"
    "minibank-go/models"
    "minibank-go/utils"

    "gorm.io/gorm"
)

func (h *Handlers) Register(w http.ResponseWriter, r *http.Request) {
    var req models.RegisterRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid request body", http.StatusBadRequest)
        return
    }

    if err := utils.ValidateStruct(req); err != nil {
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusBadRequest)
        json.NewEncoder(w).Encode(map[string]interface{}{
            "error": "Validation failed",
            "details": utils.FormatValidationError(err),
        })
        return
    }

    // Check if user already exists
    var existingUser models.User
    if err := h.db.Where("email = ? OR phone = ?", req.Email, req.Phone).First(&existingUser).Error; err == nil {
        http.Error(w, "User already exists", http.StatusConflict)
        return
    }

    // Hash password
    hashedPassword, err := utils.HashPassword(req.Password)
    if err != nil {
        http.Error(w, "Failed to hash password", http.StatusInternalServerError)
        return
    }

    // Determine if user should be admin
    isAdmin := false
    adminReason := ""

    // Method 1: Admin code provided
    if req.AdminCode != "" {
        if req.AdminCode == h.config.AdminCode {
            isAdmin = true
            adminReason = "Admin code provided"
            log.Printf("Admin user registered with admin code: %s", req.Email)
        } else {
            log.Printf("Invalid admin code provided for %s", req.Email)
            http.Error(w, "Invalid admin code", http.StatusBadRequest)
            return
        }
    }

    // Method 2: Admin email pattern (as fallback)
    if !isAdmin && strings.Contains(strings.ToLower(req.Email), "admin@") {
        isAdmin = true
        adminReason = "Admin email pattern detected"
        log.Printf("Admin user registered with admin email pattern: %s", req.Email)
    }

    // Create user
    user := models.User{
        Email:     req.Email,
        Phone:     req.Phone,
        Password:  hashedPassword,
        FirstName: req.FirstName,
        LastName:  req.LastName,
        Balance:   0,
        IsActive:  true,
        IsAdmin:   isAdmin,
        KYCStatus: "pending",
    }

    if err := h.db.Create(&user).Error; err != nil {
        log.Printf("Failed to create user %s: %v", req.Email, err)
        http.Error(w, "Failed to create user", http.StatusInternalServerError)
        return
    }

    log.Printf("User created successfully: ID=%d, Email=%s, IsAdmin=%v", user.ID, user.Email, user.IsAdmin)

    // Log audit with admin status
    auditDetails := "User registered"
    if isAdmin {
        auditDetails = "Admin user registered - " + adminReason
    }
    h.logAudit(&user.ID, "CREATE", "USER", auditDetails, r.RemoteAddr, r.UserAgent())

    // Remove password from response
    user.Password = ""

    response := map[string]interface{}{
        "message": "User registered successfully",
        "user":    user,
    }

    if isAdmin {
        response["admin_status"] = "Admin privileges granted"
        response["admin_reason"] = adminReason
    }

    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusCreated)
    json.NewEncoder(w).Encode(response)
}

func (h *Handlers) Login(w http.ResponseWriter, r *http.Request) {
    var req models.LoginRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid request body", http.StatusBadRequest)
        return
    }

    if err := utils.ValidateStruct(req); err != nil {
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusBadRequest)
        json.NewEncoder(w).Encode(map[string]interface{}{
            "error": "Validation failed",
            "details": utils.FormatValidationError(err),
        })
        return
    }

    // Find user
    var user models.User
    if err := h.db.Where("email = ?", req.Email).First(&user).Error; err != nil {
        if err == gorm.ErrRecordNotFound {
            log.Printf("Login attempt with non-existent email: %s", req.Email)
            http.Error(w, "Invalid credentials", http.StatusUnauthorized)
            return
        }
        log.Printf("Database error during login for %s: %v", req.Email, err)
        http.Error(w, "Database error", http.StatusInternalServerError)
        return
    }

    // Check password
    if !utils.CheckPasswordHash(req.Password, user.Password) {
        log.Printf("Invalid password for user: %s", req.Email)
        http.Error(w, "Invalid credentials", http.StatusUnauthorized)
        return
    }

    // Check if user is active
    if !user.IsActive {
        log.Printf("Login attempt for inactive user: %s", req.Email)
        http.Error(w, "Account is deactivated", http.StatusForbidden)
        return
    }

    log.Printf("User login: ID=%d, Email=%s, IsAdmin=%v", user.ID, user.Email, user.IsAdmin)

    // Generate token with admin status
    token, err := utils.GenerateToken(user.ID, user.Email, user.IsAdmin)
    if err != nil {
        log.Printf("Failed to generate token for user %s: %v", req.Email, err)
        http.Error(w, "Failed to generate token", http.StatusInternalServerError)
        return
    }

    // Log audit
    loginDetails := "User logged in"
    if user.IsAdmin {
        loginDetails = "Admin user logged in"
    }
    h.logAudit(&user.ID, "LOGIN", "AUTH", loginDetails, r.RemoteAddr, r.UserAgent())

    // Remove password from response
    user.Password = ""

    response := models.LoginResponse{
        Token: token,
        User:  user,
    }

    log.Printf("Login successful for %s, admin status: %v", user.Email, user.IsAdmin)

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(response)
}

// Debug endpoint to check token claims
func (h *Handlers) DebugToken(w http.ResponseWriter, r *http.Request) {
    claims := middleware.GetUserFromContext(r)
    if claims == nil {
        http.Error(w, "No user context found", http.StatusUnauthorized)
        return
    }

    // Also get fresh user data from database
    var user models.User
    if err := h.db.First(&user, claims.UserID).Error; err != nil {
        http.Error(w, "User not found in database", http.StatusNotFound)
        return
    }

    response := map[string]interface{}{
        "token_claims": map[string]interface{}{
            "user_id":  claims.UserID,
            "email":    claims.Email,
            "is_admin": claims.IsAdmin,
            "expires":  claims.ExpiresAt,
            "issued":   claims.IssuedAt,
        },
        "database_user": map[string]interface{}{
            "id":         user.ID,
            "email":      user.Email,
            "is_admin":   user.IsAdmin,
            "is_active":  user.IsActive,
            "kyc_status": user.KYCStatus,
        },
        "admin_status_match": claims.IsAdmin == user.IsAdmin,
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(response)
}