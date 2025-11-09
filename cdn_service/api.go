package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
)

func handleLoginAPI(authServiceURL string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			respondWithError(w, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}

		var authRequest AuthRequest
		if err := json.NewDecoder(r.Body).Decode(&authRequest); err != nil {
			respondWithError(w, http.StatusBadRequest, "Invalid request body")
			return
		}

		jsonData, _ := json.Marshal(authRequest)
		loginURL := fmt.Sprintf("%s/login", authServiceURL)

		resp, err := http.Post(loginURL, "application/json", bytes.NewBuffer(jsonData))
		if err != nil {
			log.Printf("Failed to connect to auth service at %s: %v", loginURL, err)
			respondWithError(w, http.StatusInternalServerError, "Authentication service unavailable")
			return
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(resp.StatusCode)
		w.Write(body)
	}
}

func handleRegisterAPI(authServiceURL string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			respondWithError(w, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}

		var authRequest AuthRequest
		if err := json.NewDecoder(r.Body).Decode(&authRequest); err != nil {
			respondWithError(w, http.StatusBadRequest, "Invalid request body")
			return
		}

		jsonData, _ := json.Marshal(authRequest)
		registerURL := fmt.Sprintf("%s/register", authServiceURL)

		resp, err := http.Post(registerURL, "application/json", bytes.NewBuffer(jsonData))
		if err != nil {
			log.Printf("Failed to connect to auth service at %s: %v", registerURL, err)
			respondWithError(w, http.StatusInternalServerError, "Authentication service unavailable")
			return
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(resp.StatusCode)
		w.Write(body)
	}
}
