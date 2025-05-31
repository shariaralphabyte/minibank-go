package handlers

import (
    "encoding/json"
    "fmt"
    "log"
    "net/http"
    "strconv"
    "time"

    "minibank-go/middleware"
    "minibank-go/models"
    "minibank-go/utils"

    "gorm.io/gorm"
)

func (h *Handlers) GetTransactions(w http.ResponseWriter, r *http.Request) {
    claims := middleware.GetUserFromContext(r)
    if claims == nil {
        http.Error(w, "Unauthorized", http.StatusUnauthorized)
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
        http.Error(w, "Failed to fetch transactions", http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(transactions)
}

func (h *Handlers) Deposit(w http.ResponseWriter, r *http.Request) {
    claims := middleware.GetUserFromContext(r)
    if claims == nil {
        http.Error(w, "Unauthorized", http.StatusUnauthorized)
        return
    }

    var req models.TransactionRequest
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

    // Check if user KYC is verified
    var user models.User
    if err := h.db.First(&user, claims.UserID).Error; err != nil {
        http.Error(w, "User not found", http.StatusNotFound)
        return
    }

    if user.KYCStatus != "verified" {
        http.Error(w, "KYC verification required", http.StatusForbidden)
        return
    }

    // Begin transaction
    tx := h.db.Begin()
    defer func() {
        if r := recover(); r != nil {
            tx.Rollback()
        }
    }()

    if err := tx.Error; err != nil {
        http.Error(w, "Failed to begin transaction", http.StatusInternalServerError)
        return
    }

    // Lock user record for update
    if err := tx.Set("gorm:query_option", "FOR UPDATE").First(&user, claims.UserID).Error; err != nil {
        tx.Rollback()
        http.Error(w, "Failed to lock user record", http.StatusInternalServerError)
        return
    }

    balanceBefore := user.Balance
    balanceAfter := balanceBefore + req.Amount

    // Update user balance
    if err := tx.Model(&user).Update("balance", balanceAfter).Error; err != nil {
        tx.Rollback()
        http.Error(w, "Failed to update balance", http.StatusInternalServerError)
        return
    }

    // Create transaction record
    transaction := models.Transaction{
        UserID:        claims.UserID,
        Type:          "deposit",
        Amount:        req.Amount,
        BalanceBefore: balanceBefore,
        BalanceAfter:  balanceAfter,
        Description:   req.Description,
        Reference:     h.generateReference(),
        Status:        "completed",
        IPAddress:     r.RemoteAddr,
    }

    if err := tx.Create(&transaction).Error; err != nil {
        tx.Rollback()
        http.Error(w, "Failed to create transaction", http.StatusInternalServerError)
        return
    }

    // Commit transaction
    if err := tx.Commit().Error; err != nil {
        http.Error(w, "Failed to commit transaction", http.StatusInternalServerError)
        return
    }

    h.logAudit(&claims.UserID, "CREATE", "TRANSACTION", 
        fmt.Sprintf("Deposit of %.2f", req.Amount), r.RemoteAddr, r.UserAgent())

    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusCreated)
    json.NewEncoder(w).Encode(map[string]interface{}{
        "message":     "Deposit successful",
        "transaction": transaction,
        "new_balance": balanceAfter,
    })
}

func (h *Handlers) Withdraw(w http.ResponseWriter, r *http.Request) {
    claims := middleware.GetUserFromContext(r)
    if claims == nil {
        http.Error(w, "Unauthorized", http.StatusUnauthorized)
        return
    }

    var req models.TransactionRequest
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

    // Check daily withdrawal limit
    if err := h.checkDailyLimit(claims.UserID, req.Amount, "withdraw"); err != nil {
        http.Error(w, err.Error(), http.StatusForbidden)
        return
    }

    // Begin transaction
    tx := h.db.Begin()
    defer func() {
        if r := recover(); r != nil {
            tx.Rollback()
        }
    }()

    if err := tx.Error; err != nil {
        http.Error(w, "Failed to begin transaction", http.StatusInternalServerError)
        return
    }

    // Lock user record for update
    var user models.User
    if err := tx.Set("gorm:query_option", "FOR UPDATE").First(&user, claims.UserID).Error; err != nil {
        tx.Rollback()
        http.Error(w, "Failed to lock user record", http.StatusInternalServerError)
        return
    }

    if user.KYCStatus != "verified" {
        tx.Rollback()
        http.Error(w, "KYC verification required", http.StatusForbidden)
        return
    }

    if user.Balance < req.Amount {
        tx.Rollback()
        http.Error(w, "Insufficient balance", http.StatusBadRequest)
        return
    }

    balanceBefore := user.Balance
    balanceAfter := balanceBefore - req.Amount

    // Update user balance
    if err := tx.Model(&user).Update("balance", balanceAfter).Error; err != nil {
        tx.Rollback()
        http.Error(w, "Failed to update balance", http.StatusInternalServerError)
        return
    }

    // Create transaction record
    transaction := models.Transaction{
        UserID:        claims.UserID,
        Type:          "withdraw",
        Amount:        req.Amount,
        BalanceBefore: balanceBefore,
        BalanceAfter:  balanceAfter,
        Description:   req.Description,
        Reference:     h.generateReference(),
        Status:        "completed",
        IPAddress:     r.RemoteAddr,
    }

    if err := tx.Create(&transaction).Error; err != nil {
        tx.Rollback()
        http.Error(w, "Failed to create transaction", http.StatusInternalServerError)
        return
    }

    // Commit transaction
    if err := tx.Commit().Error; err != nil {
        http.Error(w, "Failed to commit transaction", http.StatusInternalServerError)
        return
    }

    h.logAudit(&claims.UserID, "CREATE", "TRANSACTION", 
        fmt.Sprintf("Withdrawal of %.2f", req.Amount), r.RemoteAddr, r.UserAgent())

    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusCreated)
    json.NewEncoder(w).Encode(map[string]interface{}{
        "message":     "Withdrawal successful",
        "transaction": transaction,
        "new_balance": balanceAfter,
    })
}

func (h *Handlers) Transfer(w http.ResponseWriter, r *http.Request) {
    claims := middleware.GetUserFromContext(r)
    if claims == nil {
        http.Error(w, "Unauthorized", http.StatusUnauthorized)
        return
    }

    var req models.TransferRequest
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

    // Check AML rules (rapid small transfers)
    if err := h.checkAMLRules(claims.UserID, req.Amount); err != nil {
        http.Error(w, err.Error(), http.StatusForbidden)
        return
    }

    // Check daily transfer limit
    if err := h.checkDailyLimit(claims.UserID, req.Amount, "transfer"); err != nil {
        http.Error(w, err.Error(), http.StatusForbidden)
        return
    }

    // Find recipient
    var toUser models.User
    if err := h.db.Where("email = ?", req.ToUserEmail).First(&toUser).Error; err != nil {
        if err == gorm.ErrRecordNotFound {
            http.Error(w, "Recipient not found", http.StatusNotFound)
            return
        }
        http.Error(w, "Database error", http.StatusInternalServerError)
        return
    }

    if toUser.ID == claims.UserID {
        http.Error(w, "Cannot transfer to yourself", http.StatusBadRequest)
        return
    }

    if toUser.KYCStatus != "verified" {
        http.Error(w, "Recipient KYC not verified", http.StatusForbidden)
        return
    }

    // Begin transaction
    tx := h.db.Begin()
    defer func() {
        if r := recover(); r != nil {
            tx.Rollback()
        }
    }()

    if err := tx.Error; err != nil {
        http.Error(w, "Failed to begin transaction", http.StatusInternalServerError)
        return
    }

    // Lock both user records (ordered by ID to prevent deadlock)
    var fromUser, recipientUser models.User
    if claims.UserID < toUser.ID {
        if err := tx.Set("gorm:query_option", "FOR UPDATE").First(&fromUser, claims.UserID).Error; err != nil {
            tx.Rollback()
            http.Error(w, "Failed to lock sender record", http.StatusInternalServerError)
            return
        }
        if err := tx.Set("gorm:query_option", "FOR UPDATE").First(&recipientUser, toUser.ID).Error; err != nil {
            tx.Rollback()
            http.Error(w, "Failed to lock recipient record", http.StatusInternalServerError)
            return
        }
    } else {
        if err := tx.Set("gorm:query_option", "FOR UPDATE").First(&recipientUser, toUser.ID).Error; err != nil {
            tx.Rollback()
            http.Error(w, "Failed to lock recipient record", http.StatusInternalServerError)
            return
        }
        if err := tx.Set("gorm:query_option", "FOR UPDATE").First(&fromUser, claims.UserID).Error; err != nil {
            tx.Rollback()
            http.Error(w, "Failed to lock sender record", http.StatusInternalServerError)
            return
        }
    }

    if fromUser.KYCStatus != "verified" {
        tx.Rollback()
        http.Error(w, "KYC verification required", http.StatusForbidden)
        return
    }

    if fromUser.Balance < req.Amount {
        tx.Rollback()
        http.Error(w, "Insufficient balance", http.StatusBadRequest)
        return
    }

    // Update balances
    fromBalanceBefore := fromUser.Balance
    fromBalanceAfter := fromBalanceBefore - req.Amount
    toBalanceBefore := recipientUser.Balance
    toBalanceAfter := toBalanceBefore + req.Amount

    if err := tx.Model(&fromUser).Update("balance", fromBalanceAfter).Error; err != nil {
        tx.Rollback()
        http.Error(w, "Failed to update sender balance", http.StatusInternalServerError)
        return
    }

    if err := tx.Model(&recipientUser).Update("balance", toBalanceAfter).Error; err != nil {
        tx.Rollback()
        http.Error(w, "Failed to update recipient balance", http.StatusInternalServerError)
        return
    }

    // Generate unique references for both transactions
    senderRef := h.generateReference()
    recipientRef := h.generateReference()

    // Create sender transaction
    senderTxn := models.Transaction{
        UserID:        claims.UserID,
        Type:          "transfer_out",
        Amount:        req.Amount,
        BalanceBefore: fromBalanceBefore,
        BalanceAfter:  fromBalanceAfter,
        ToUserID:      &toUser.ID,
        Description:   req.Description,
        Reference:     senderRef,
        Status:        "completed",
        IPAddress:     r.RemoteAddr,
    }

    // Create recipient transaction
    recipientTxn := models.Transaction{
        UserID:        toUser.ID,
        Type:          "transfer_in",
        Amount:        req.Amount,
        BalanceBefore: toBalanceBefore,
        BalanceAfter:  toBalanceAfter,
        FromUserID:    &claims.UserID,
        Description:   req.Description,
        Reference:     recipientRef,
        Status:        "completed",
        IPAddress:     r.RemoteAddr,
    }

    if err := tx.Create(&senderTxn).Error; err != nil {
        tx.Rollback()
        http.Error(w, "Failed to create sender transaction", http.StatusInternalServerError)
        return
    }

    if err := tx.Create(&recipientTxn).Error; err != nil {
        tx.Rollback()
        log.Printf("Failed to create recipient transaction: %v", err)
        log.Printf("Transaction details: %+v", recipientTxn)
        log.Printf("Database state: %+v", tx.Statement)
        http.Error(w, fmt.Sprintf("Failed to create recipient transaction: %v", err), http.StatusInternalServerError)
        return
    }

    // Commit transaction
    if err := tx.Commit().Error; err != nil {
        http.Error(w, "Failed to commit transaction", http.StatusInternalServerError)
        return
    }

    h.logAudit(&claims.UserID, "CREATE", "TRANSACTION", 
        fmt.Sprintf("Transfer of %.2f to %s", req.Amount, req.ToUserEmail), r.RemoteAddr, r.UserAgent())

    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusCreated)
    json.NewEncoder(w).Encode(map[string]interface{}{
        "message":     "Transfer successful",
        "transaction": senderTxn,
        "new_balance": fromBalanceAfter,
    })
}

func (h *Handlers) checkDailyLimit(userID uint, amount float64, txnType string) error {
    today := time.Now().Format("2006-01-02")
    
    var totalToday float64
    var txnTypes []string
    
    if txnType == "transfer" {
        txnTypes = []string{"transfer_out"}
    } else {
        txnTypes = []string{txnType}
    }
    
    if err := h.db.Model(&models.Transaction{}).
        Select("COALESCE(SUM(amount), 0)").
        Where("user_id = ? AND type IN ? AND DATE(created_at) = ?", userID, txnTypes, today).
        Scan(&totalToday).Error; err != nil {
        return fmt.Errorf("failed to check daily limit")
    }
    
    dailyLimit := 50000.0
    if totalToday+amount > dailyLimit {
        return fmt.Errorf("daily transaction limit exceeded")
    }
    
    return nil
}

func (h *Handlers) checkAMLRules(userID uint, amount float64) error {
    // Check for rapid small transfers (AML rule)
    fiveMinutesAgo := time.Now().Add(-5 * time.Minute)
    
    var recentCount int64
    if err := h.db.Model(&models.Transaction{}).
        Where("user_id = ? AND type = ? AND amount < ? AND created_at > ?", 
            userID, "transfer_out", 1000, fiveMinutesAgo).
        Count(&recentCount).Error; err != nil {
        return fmt.Errorf("failed to check AML rules")
    }
    
    if recentCount >= 5 && amount < 1000 {
        return fmt.Errorf("suspicious activity detected - multiple small transfers")
    }
    
    return nil
}