package utils

import (
    "fmt"
    "log"
    "time"

    "github.com/golang-jwt/jwt/v5"
)

var jwtSecret []byte

type Claims struct {
    UserID  uint   `json:"user_id"`
    Email   string `json:"email"`
    IsAdmin bool   `json:"is_admin"`
    jwt.RegisteredClaims
}

// InitializeJWT sets up the JWT secret
func InitializeJWT(secret string) error {
    if len(secret) < 32 {
        log.Printf("WARNING: JWT secret should be at least 32 characters for security, got %d", len(secret))
    }
    jwtSecret = []byte(secret)
    log.Println("JWT initialized successfully")
    return nil
}

func GenerateToken(userID uint, email string, isAdmin bool) (string, error) {
    if jwtSecret == nil {
        return "", fmt.Errorf("JWT secret not initialized")
    }

    claims := Claims{
        UserID:  userID,
        Email:   email,
        IsAdmin: isAdmin,
        RegisteredClaims: jwt.RegisteredClaims{
            ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
            IssuedAt:  jwt.NewNumericDate(time.Now()),
            Issuer:    "minibank-go",
        },
    }

    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    signedToken, err := token.SignedString(jwtSecret)
    if err != nil {
        return "", fmt.Errorf("failed to sign token: %v", err)
    }

    log.Printf("Generated JWT token for user %d (%s)", userID, email)
    return signedToken, nil
}

func ValidateToken(tokenString string) (*Claims, error) {
    if jwtSecret == nil {
        return nil, fmt.Errorf("JWT secret not initialized")
    }

    claims := &Claims{}
    token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
        // Verify signing method
        if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
            return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
        }
        return jwtSecret, nil
    })

    if err != nil {
        return nil, fmt.Errorf("failed to parse token: %v", err)
    }

    if !token.Valid {
        return nil, fmt.Errorf("invalid token")
    }

    // Check expiration
    if claims.ExpiresAt != nil && claims.ExpiresAt.Time.Before(time.Now()) {
        return nil, fmt.Errorf("token expired")
    }

    return claims, nil
}