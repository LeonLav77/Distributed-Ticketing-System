package main

import "github.com/golang-jwt/jwt/v5"

type ErrorResponse struct {
	Error string `json:"error"`
}

type Claims struct {
	Username string `json:"username"`
	UserID   int    `json:"user_id"`
	jwt.RegisteredClaims
}

type PageData struct {
	APIBaseURL     string
	AuthServiceURL string
	FrontendURL    string
	DirectusAPIURL string
	WebsocketURL   string
}

type AuthRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type AuthResponse struct {
	Token   string `json:"token"`
	Message string `json:"message"`
	UserID  int    `json:"user_id,omitempty"`
}
