package main

import (
	"log"
	"os"

	"github.com/golang-jwt/jwt/v5"
)

type ErrorResponse struct {
	Error string `json:"error"`
}

type UserClaims struct {
	Username string `json:"username"`
	UserID   int    `json:"user_id"`
	jwt.RegisteredClaims
}

func getJWTSecret() []byte {
	secret := os.Getenv("JWT_SECRET")
	return []byte(secret)
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
