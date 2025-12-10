package main

import (
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

		claims := &Claims{}
		token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
			return getJWTSecret(), nil
		})

		if err != nil || !token.Valid {
			loginURL := os.Getenv("LOGIN_URL")
			http.Redirect(w, r, loginURL, http.StatusSeeOther)
			return
		}

		next.ServeHTTP(w, r)
	}
}
