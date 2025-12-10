package main

import (
	"log"
	"net/http"
	"os"

	"github.com/golang-jwt/jwt/v5"
)

func getJWTSecret() []byte {
	secret := os.Getenv("JWT_SECRET")
	return []byte(secret)
}

func authenticateJWT(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var tokenString string

		authHeader := r.Header.Get("Authorization")
		if authHeader != "" {
			if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
				tokenString = authHeader[7:]
			}
		}

		if tokenString == "" {
			cookie, err := r.Cookie("auth_token")
			if err == nil {
				tokenString = cookie.Value
			}
		}

		if tokenString == "" {
			loginURL := os.Getenv("LOGIN_URL")
			http.Redirect(w, r, loginURL, http.StatusSeeOther)
			return
		}

		claims := UserClaims{}
		token, err := jwt.ParseWithClaims(tokenString, &claims, func(token *jwt.Token) (any, error) {
			return getJWTSecret(), nil
		})

		if err != nil || token == nil || !token.Valid {
			log.Printf("Authentication failed - error: %v, username attempted: %s", err, claims.Username)
			loginURL := os.Getenv("LOGIN_URL")
			http.Redirect(w, r, loginURL, http.StatusSeeOther)
			return
		}

		next.ServeHTTP(w, r)
	}
}

func extractUserIDFromJWT(tokenString string) int {
	claims := UserClaims{}

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
