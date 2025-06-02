package handlers

import (
    "encoding/json"
    "fmt"
    "net/http"
    "strconv"
    "time"

    "gorm.io/gorm"

    "github.com/google/uuid"

    "minibank-go/config"
    "minibank-go/models"
    "minibank-go/middleware"
    "minibank-go/utils"
)

// ErrorResponse represents a standardized error response
// Status: HTTP status code
// Error: Error message
// Details: Additional details about the error
// Timestamp: When the error occurred
type ErrorResponse struct {
    Status   int         `json:"status"`
    Error    string      `json:"error"`
    Details  interface{} `json:"details,omitempty"`
    Timestamp time.Time  `json:"timestamp"`
}

// SendError sends a standardized error response
func sendError(w http.ResponseWriter, status int, err string, details interface{}) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    json.NewEncoder(w).Encode(ErrorResponse{
        Status:   status,
        Error:    err,
        Details:  details,
        Timestamp: time.Now(),
    })
}

type Handlers struct {
    db     *gorm.DB
    config *config.Config
}

// generateReference generates a unique transaction reference
func (h *Handlers) generateReference() string {
    return uuid.New().String()
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

// Check if user is admin
func (h *Handlers) isUserAdmin(claims *utils.Claims) bool {
    if claims == nil {
        return false
    }
    return claims.IsAdmin
}

// Send unauthorized response for admin endpoints
func (h *Handlers) sendUnauthorized(w http.ResponseWriter, r *http.Request) {
    claims := middleware.GetUserFromContext(r)
    if claims == nil {
        sendError(w, http.StatusUnauthorized, "Invalid token", nil)
        return
    }

    sendError(w, http.StatusUnauthorized, "Unauthorized", map[string]interface{}{
        "error": "Admin access required",
        "user_id": claims.UserID,
        "email": claims.Email,
        "is_admin": claims.IsAdmin,
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

// Transaction methods
func (h *Handlers) GetTransactions(w http.ResponseWriter, r *http.Request) {
    claims := middleware.GetUserFromContext(r)
    if claims == nil {
        sendError(w, http.StatusUnauthorized, "Invalid or missing token", nil)
        return
    }

    page, _ := strconv.Atoi(r.URL.Query().Get("page"))
    if page <= 0 {
        page = 1
    }
    limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
    if limit <= 0 || limit > 100 {
        limit = 20
    }
    offset := (page - 1) * limit

    var transactions []models.Transaction
    if err := h.db.Where("user_id = ?", claims.UserID).
        Order("created_at DESC").
        Limit(limit).
        Offset(offset).
        Find(&transactions).Error; err != nil {
        sendError(w, http.StatusInternalServerError, "Failed to fetch transactions", err.Error())
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(transactions)
}

// Check daily transaction limits
func (h *Handlers) checkDailyLimit(userID uint, amount float64, txnType string) error {
    var dailyLimit float64
    switch txnType {
    case "deposit":
        dailyLimit = h.config.TransactionLimits.DailyDepositLimit
    case "withdraw":
        dailyLimit = h.config.TransactionLimits.DailyWithdrawLimit
    case "transfer":
        dailyLimit = h.config.TransactionLimits.DailyTransferLimit
    default:
        return fmt.Errorf("invalid transaction type: %s", txnType)
    }

    var totalToday float64
    if err := h.db.Model(&models.Transaction{}).
        Where("user_id = ? AND created_at >= ? AND type = ?",
            userID, time.Now().Format("2006-01-02 00:00:00"), txnType).
        Select("COALESCE(SUM(amount), 0)").
        Scan(&totalToday).Error; err != nil {
        return fmt.Errorf("failed to calculate daily limit: %w", err)
    }

    if totalToday+amount > dailyLimit {
        return fmt.Errorf("daily limit exceeded: %.2f/%.2f", totalToday+amount, dailyLimit)
    }
    return nil
}

// Check AML rules
func (h *Handlers) checkAMLRules(userID uint, amount float64) error {
    // Get user's total transactions in last 30 days
    var totalLast30Days float64
    if err := h.db.Model(&models.Transaction{}).
        Where("user_id = ? AND created_at >= ?",
            userID, time.Now().AddDate(0, -1, 0).Format("2006-01-02 00:00:00")).
        Select("COALESCE(SUM(amount), 0)").
        Scan(&totalLast30Days).Error; err != nil {
        return fmt.Errorf("failed to calculate 30-day total: %w", err)
    }

    // Check for suspicious patterns
    if totalLast30Days > h.config.AMLRules.MonthlyThreshold {
        return fmt.Errorf("suspicious transaction pattern detected: total amount exceeds monthly threshold")
    }

    // Check for rapid transactions
    var recentTxns []models.Transaction
    if err := h.db.Where("user_id = ? AND created_at >= ?",
        userID, time.Now().Add(-time.Hour*24).Format("2006-01-02 00:00:00")).
        Find(&recentTxns).Error; err != nil {
        return fmt.Errorf("failed to check recent transactions: %w", err)
    }

    if len(recentTxns) > h.config.AMLRules.DailyTransactionLimit {
        return fmt.Errorf("too many transactions in 24 hours")
    }

    return nil
}

// Deposit handler
func (h *Handlers) Deposit(w http.ResponseWriter, r *http.Request) {
    claims := middleware.GetUserFromContext(r)
    if claims == nil {
        sendError(w, http.StatusUnauthorized, "Invalid or missing token", nil)
        return
    }

    var req models.DepositRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        sendError(w, http.StatusBadRequest, "Invalid request body", err.Error())
        return
    }

    if err := utils.ValidateStruct(req); err != nil {
        errors := utils.FormatValidationError(err)
        sendError(w, http.StatusBadRequest, "Validation failed", errors)
        return
    }

    // Check daily limit
    if err := h.checkDailyLimit(claims.UserID, req.Amount, "deposit"); err != nil {
        sendError(w, http.StatusBadRequest, err.Error(), nil)
        return
    }

    // Check AML rules
    if err := h.checkAMLRules(claims.UserID, req.Amount); err != nil {
        sendError(w, http.StatusBadRequest, err.Error(), nil)
        return
    }

    // Begin transaction
    tx := h.db.Begin()
    defer func() {
        if r := recover(); r != nil {
            tx.Rollback()
        }
    }()

    // Lock user record for update
    var user models.User
    if err := tx.Set("gorm:query_option", "FOR UPDATE").First(&user, claims.UserID).Error; err != nil {
        tx.Rollback()
        sendError(w, http.StatusInternalServerError, "Failed to lock user record", err.Error())
        return
    }

    // Update balance
    user.Balance += req.Amount

    if err := tx.Save(&user).Error; err != nil {
        tx.Rollback()
        sendError(w, http.StatusInternalServerError, "Failed to update balance", err.Error())
        return
    }

    // Create transaction record
    txn := models.Transaction{
        UserID:        claims.UserID,
        Type:          "deposit",
        Amount:        req.Amount,
        BalanceBefore: user.Balance - req.Amount,
        BalanceAfter:  user.Balance,
        Description:   req.Description,
        Reference:     h.generateReference(),
    }

    if err := tx.Create(&txn).Error; err != nil {
        tx.Rollback()
        sendError(w, http.StatusInternalServerError, "Failed to create transaction record", err.Error())
        return
    }

    // Log audit
    h.logAudit(&claims.UserID, "deposit", "transaction", fmt.Sprintf("Deposited %f", req.Amount), r.RemoteAddr, r.UserAgent())

    tx.Commit()

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]interface{}{
        "message": "Deposit successful",
        "transaction": txn,
        "new_balance": user.Balance,
    })
}

// Withdraw handler
func (h *Handlers) Withdraw(w http.ResponseWriter, r *http.Request) {
    claims := middleware.GetUserFromContext(r)
    if claims == nil {
        sendError(w, http.StatusUnauthorized, "Invalid or missing token", nil)
        return
    }

    var req models.WithdrawRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        sendError(w, http.StatusBadRequest, "Invalid request body", err.Error())
        return
    }

    if err := utils.ValidateStruct(req); err != nil {
        errors := utils.FormatValidationError(err)
        sendError(w, http.StatusBadRequest, "Validation failed", errors)
        return
    }

    // Check daily limit
    if err := h.checkDailyLimit(claims.UserID, req.Amount, "withdraw"); err != nil {
        sendError(w, http.StatusBadRequest, err.Error(), nil)
        return
    }

    // Check AML rules
    if err := h.checkAMLRules(claims.UserID, req.Amount); err != nil {
        sendError(w, http.StatusBadRequest, err.Error(), nil)
        return
    }

    // Begin transaction
    tx := h.db.Begin()
    defer func() {
        if r := recover(); r != nil {
            tx.Rollback()
        }
    }()

    // Lock user record for update
    var user models.User
    if err := tx.Set("gorm:query_option", "FOR UPDATE").First(&user, claims.UserID).Error; err != nil {
        tx.Rollback()
        sendError(w, http.StatusInternalServerError, "Failed to lock user record", err.Error())
        return
    }

    // Check sufficient balance
    if user.Balance < req.Amount {
        tx.Rollback()
        sendError(w, http.StatusForbidden, "Insufficient balance", nil)
        return
    }

    // Update balance
    user.Balance -= req.Amount

    if err := tx.Save(&user).Error; err != nil {
        tx.Rollback()
        sendError(w, http.StatusInternalServerError, "Failed to update balance", err.Error())
        return
    }

    // Create transaction record
    txn := models.Transaction{
        UserID:        claims.UserID,
        Type:          "withdraw",
        Amount:        req.Amount,
        BalanceBefore: user.Balance + req.Amount,
        BalanceAfter:  user.Balance,
        Description:   req.Description,
        Reference:     h.generateReference(),
    }

    if err := tx.Create(&txn).Error; err != nil {
        tx.Rollback()
        sendError(w, http.StatusInternalServerError, "Failed to create transaction record", err.Error())
        return
    }

    // Log audit
    h.logAudit(&claims.UserID, "withdraw", "transaction", fmt.Sprintf("Withdrew %f", req.Amount), r.RemoteAddr, r.UserAgent())

    tx.Commit()

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]interface{}{
        "message": "Withdrawal successful",
        "transaction": txn,
        "new_balance": user.Balance,
    })
}

// Transfer handler
func (h *Handlers) Transfer(w http.ResponseWriter, r *http.Request) {
    claims := middleware.GetUserFromContext(r)
    if claims == nil {
        sendError(w, http.StatusUnauthorized, "Invalid or missing token", nil)
        return
    }

    var req models.TransferRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        sendError(w, http.StatusBadRequest, "Invalid request body", err.Error())
        return
    }

    if err := utils.ValidateStruct(req); err != nil {
        errors := utils.FormatValidationError(err)
        sendError(w, http.StatusBadRequest, "Validation failed", errors)
        return
    }

    // Check daily limit
    if err := h.checkDailyLimit(claims.UserID, req.Amount, "transfer"); err != nil {
        sendError(w, http.StatusBadRequest, err.Error(), nil)
        return
    }

    // Check AML rules
    if err := h.checkAMLRules(claims.UserID, req.Amount); err != nil {
        sendError(w, http.StatusBadRequest, err.Error(), nil)
        return
    }

    // Begin transaction
    tx := h.db.Begin()
    defer func() {
        if r := recover(); r != nil {
            tx.Rollback()
        }
    }()

    // Lock both user records for update
    var fromUser, toUser models.User
    if err := tx.Set("gorm:query_option", "FOR UPDATE").First(&fromUser, claims.UserID).Error; err != nil {
        tx.Rollback()
        sendError(w, http.StatusInternalServerError, "Failed to lock sender record", err.Error())
        return
    }

    if err := tx.Set("gorm:query_option", "FOR UPDATE").First(&toUser, req.ToUserID).Error; err != nil {
        tx.Rollback()
        if err == gorm.ErrRecordNotFound {
            sendError(w, http.StatusNotFound, "Recipient user not found", map[string]string{"error_detail": err.Error()})
        } else {
            sendError(w, http.StatusInternalServerError, "Error fetching recipient user", map[string]string{"error_detail": err.Error()})
        }
        return
    }

    // Check sufficient balance
    if fromUser.Balance < req.Amount {
        tx.Rollback()
        sendError(w, http.StatusForbidden, "Insufficient balance", nil)
        return
    }

    // Update balances
    fromUser.Balance -= req.Amount
    toUser.Balance += req.Amount

    if err := tx.Save(&fromUser).Error; err != nil {
        tx.Rollback()
        sendError(w, http.StatusInternalServerError, "Failed to update sender balance", err.Error())
        return
    }

    if err := tx.Save(&toUser).Error; err != nil {
        tx.Rollback()
        sendError(w, http.StatusInternalServerError, "Failed to update recipient balance", err.Error())
        return
    }

    // Create transaction records
    senderTxn := models.Transaction{
        UserID:        claims.UserID,
        Type:          "transfer_out",
        Amount:        req.Amount,
        BalanceBefore: fromUser.Balance + req.Amount,
        BalanceAfter:  fromUser.Balance,
        ToUserID:      &toUser.ID,
        Description:   req.Description,
        Reference:     h.generateReference(),
    }

    receiverTxn := models.Transaction{
        UserID:        toUser.ID,
        Type:          "transfer_in",
        Amount:        req.Amount,
        BalanceBefore: toUser.Balance - req.Amount,
        BalanceAfter:  toUser.Balance,
        FromUserID:    &claims.UserID,
        Description:   req.Description,
        Reference:     senderTxn.Reference,
    }

    if err := tx.Create(&senderTxn).Error; err != nil {
        tx.Rollback()
        sendError(w, http.StatusInternalServerError, "Failed to create sender transaction record", err.Error())
        return
    }

    if err := tx.Create(&receiverTxn).Error; err != nil {
        tx.Rollback()
        sendError(w, http.StatusInternalServerError, "Failed to create receiver transaction record", err.Error())
        return
    }

    // Log audit
    h.logAudit(&claims.UserID, "transfer_out", "transaction", fmt.Sprintf("Transferred %f to user %d", req.Amount, req.ToUserID), r.RemoteAddr, r.UserAgent())
    h.logAudit(&toUser.ID, "transfer_in", "transaction", fmt.Sprintf("Received %f from user %d", req.Amount, claims.UserID), r.RemoteAddr, r.UserAgent())

    tx.Commit()

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]interface{}{
        "message": "Transfer successful",
        "transaction": senderTxn,
        "new_balance": fromUser.Balance,
    })
}

// VerifyUser handler
func (h *Handlers) VerifyUser(w http.ResponseWriter, r *http.Request) {
    var req struct {
        UserID uint `json:"user_id" validate:"required,gt=0"`
        Status string `json:"status" validate:"required,oneof=verified rejected"`
    }

    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        sendError(w, http.StatusBadRequest, "Invalid request body", err.Error())
        return
    }

    if err := utils.ValidateStruct(req); err != nil {
        sendError(w, http.StatusBadRequest, "Validation failed", utils.FormatValidationError(err))
        return
    }

    var user models.User
    if err := h.db.First(&user, req.UserID).Error; err != nil {
        sendError(w, http.StatusNotFound, "User not found", nil)
        return
    }

    user.Verified = req.Status == "verified"
    if err := h.db.Save(&user).Error; err != nil {
        sendError(w, http.StatusInternalServerError, "Failed to update user verification status", err.Error())
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]interface{}{
        "message": "User verification status updated",
        "user":    user,
    })
}

// DebugToken handler retrieves token claims and database user state
func (h *Handlers) DebugToken(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserFromContext(r)
	if claims == nil {
		sendError(w, http.StatusUnauthorized, "Invalid or missing token", "No claims found in context, ensure JWT middleware is active and token is valid.")
		return
	}

	var user models.User
	if err := h.db.First(&user, claims.UserID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			sendError(w, http.StatusNotFound, "User not found in database", fmt.Sprintf("User ID %d from token not found in DB.", claims.UserID))
		} else {
			sendError(w, http.StatusInternalServerError, "Failed to fetch user from database", err.Error())
		}
		return
	}

	// Clear sensitive information
	user.Password = ""

	adminStatusMatch := claims.IsAdmin == user.IsAdmin

	response := map[string]interface{}{
		"token_claims":       claims,
		"database_user":      user,
		"admin_status_match": adminStatusMatch,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		// If encoding fails, log it and send a generic server error.
		// This is a fallback, ideally json.NewEncoder should not fail with map[string]interface{}
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		// Log the actual error server-side e.g., log.Printf("Failed to encode DebugToken response: %v", err)
	}
}
