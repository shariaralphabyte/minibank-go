package middleware

import (
    "context"
    "encoding/json"
    "log"
    "net/http"
    "strings"

    "minibank-go/utils"
)

type contextKey string

const UserContextKey contextKey = "user"

func JWTAuth(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        authHeader := r.Header.Get("Authorization")
        if authHeader == "" {
            log.Printf("No Authorization header found for %s", r.URL.Path)
            http.Error(w, "Authorization header required", http.StatusUnauthorized)
            return
        }

        bearerToken := strings.Split(authHeader, " ")
        if len(bearerToken) != 2 || bearerToken[0] != "Bearer" {
            log.Printf("Invalid Authorization header format for %s", r.URL.Path)
            http.Error(w, "Invalid authorization header format", http.StatusUnauthorized)
            return
        }

        claims, err := utils.ValidateToken(bearerToken[1])
        if err != nil {
            log.Printf("Token validation failed for %s: %v", r.URL.Path, err)
            http.Error(w, "Invalid token", http.StatusUnauthorized)
            return
        }

        log.Printf("Token validated for user %d (%s), admin: %v", claims.UserID, claims.Email, claims.IsAdmin)

        ctx := context.WithValue(r.Context(), UserContextKey, claims)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}

func AdminAuth(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        claims, ok := r.Context().Value(UserContextKey).(*utils.Claims)
        if !ok {
            log.Printf("No user claims found in context for %s", r.URL.Path)
            w.Header().Set("Content-Type", "application/json")
            w.WriteHeader(http.StatusUnauthorized)
            json.NewEncoder(w).Encode(map[string]string{
                "error": "Unauthorized - No user context",
            })
            return
        }

        log.Printf("Admin check for user %d (%s): is_admin = %v", claims.UserID, claims.Email, claims.IsAdmin)

        if !claims.IsAdmin {
            log.Printf("User %d attempted to access admin endpoint %s without admin privileges", 
                claims.UserID, r.URL.Path)
            w.Header().Set("Content-Type", "application/json")
            w.WriteHeader(http.StatusForbidden)
            json.NewEncoder(w).Encode(map[string]string{
                "error": "Admin access required",
                "message": "This endpoint requires admin privileges",
            })
            return
        }

        log.Printf("Admin access granted for user %d (%s) to %s", claims.UserID, claims.Email, r.URL.Path)
        next.ServeHTTP(w, r)
    })
}

func GetUserFromContext(r *http.Request) *utils.Claims {
    if claims, ok := r.Context().Value(UserContextKey).(*utils.Claims); ok {
        return claims
    }
    return nil
}