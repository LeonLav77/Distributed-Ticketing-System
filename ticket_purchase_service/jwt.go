package main

import (
	"log"
	"net/http"

	"github.com/golang-jwt/jwt/v5"
)

type ErrorResponse struct {
	Error string `json:"error"`
}

type Claims struct {
	Username string `json:"username"`
	UserID   int    `json:"user_id"`
	jwt.RegisteredClaims
}

func getJWTSecret() []byte {
	secret := getEnv("JWT_SECRET", "kaodajemorskavila")
	return []byte(secret)
}

func authenticateJWT(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var tokenString string

		// Try to get token from Authorization header first (for API calls)
		authHeader := r.Header.Get("Authorization")
		if authHeader != "" {
			if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
				tokenString = authHeader[7:]
			}
		}

		// If not in header, try cookie (for page loads)
		if tokenString == "" {
			cookie, err := r.Cookie("auth_token")
			if err == nil {
				tokenString = cookie.Value
			}
		}

		// If still no token, redirect to login
		if tokenString == "" {
			loginURL := getEnv("LOGIN_URL", "/login")
			http.Redirect(w, r, loginURL, http.StatusSeeOther)
			return
		}

		claims := Claims{}
		token, err := jwt.ParseWithClaims(tokenString, &claims, func(token *jwt.Token) (any, error) {
			return getJWTSecret(), nil
		})

		if err != nil || token == nil || !token.Valid {
			log.Printf("Authentication failed - error: %v, username attempted: %s", err, claims.Username)
			loginURL := getEnv("LOGIN_URL", "/login")
			http.Redirect(w, r, loginURL, http.StatusSeeOther)
			return
		}

		next.ServeHTTP(w, r)
	}
}

func extractUserIDFromJWT(tokenString string) int {
	claims := Claims{}

	if tokenString != "" {
		if len(tokenString) > 7 && tokenString[:7] == "Bearer " {
			tokenString = tokenString[7:]
		}
	}

	token, err := jwt.ParseWithClaims(tokenString, &claims, func(token *jwt.Token) (any, error) {
		return getJWTSecret(), nil
	})

	if err != nil || token == nil || !token.Valid {
		log.Printf("Failed to extract user ID from JWT - error: %v", err)
		return 0
	}

	return claims.UserID
}
