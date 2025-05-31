package middleware

import (
    "net/http"
    "sync"
    "time"

    "golang.org/x/time/rate"
)

type visitor struct {
    limiter  *rate.Limiter
    lastSeen time.Time
}

var (
    visitors = make(map[string]*visitor)
    mtx      sync.RWMutex
)

func getVisitor(ip string) *rate.Limiter {
    mtx.RLock()
    v, exists := visitors[ip]
    mtx.RUnlock()

    if !exists {
        limiter := rate.NewLimiter(10, 50) // 10 requests per second, burst of 50
        mtx.Lock()
        visitors[ip] = &visitor{limiter, time.Now()}
        mtx.Unlock()
        return limiter
    }

    v.lastSeen = time.Now()
    return v.limiter
}

func cleanupVisitors() {
    for {
        time.Sleep(time.Minute)
        mtx.Lock()
        for ip, v := range visitors {
            if time.Since(v.lastSeen) > 3*time.Minute {
                delete(visitors, ip)
            }
        }
        mtx.Unlock()
    }
}

func init() {
    go cleanupVisitors()
}

func RateLimit(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        limiter := getVisitor(r.RemoteAddr)
        if !limiter.Allow() {
            http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
            return
        }

        next.ServeHTTP(w, r)
    })
}