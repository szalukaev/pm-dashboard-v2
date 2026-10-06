package middleware

import (
	"net/http"
	"os"
	"strings"
)

func CORSMiddleware(next http.Handler) http.Handler {
	// Allowed origins — comma-separated from env, or default local dev
	allowedOrigins := map[string]bool{
		"http://localhost:3000": true,
		"http://localhost:8080": true,
		"http://localhost:82":   true,
	}

	if origins := os.Getenv("CORS_ORIGINS"); origins != "" {
		allowedOrigins = make(map[string]bool)
		for _, o := range strings.Split(origins, ",") {
			allowedOrigins[strings.TrimSpace(o)] = true
		}
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if allowedOrigins[origin] {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			// Vary: Origin — responses differ per Origin (caching correctness).
			w.Header().Add("Vary", "Origin")
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Credentials", "true")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// maxJSONBodyBytes is the maximum accepted JSON request body size.
const maxJSONBodyBytes = 1 << 20 // 1 MiB

// LimitJSONBody wraps r.Body with a size cap for application/json requests.
func LimitJSONBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Body != nil && strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
			r.Body = http.MaxBytesReader(w, r.Body, maxJSONBodyBytes)
		}
		next.ServeHTTP(w, r)
	})
}
