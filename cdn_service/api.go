package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
)

func handleLoginAPI(w http.ResponseWriter, r *http.Request) {
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

func handleRegisterAPI(w http.ResponseWriter, r *http.Request) {
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

func handleGetAvailableTickets(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondWithError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	ticketsURL := fmt.Sprintf("%s/get-available-tickets?%s", ticketServiceURL, r.URL.RawQuery)
	
	req, err := http.NewRequest(http.MethodGet, ticketsURL, nil)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to create request")
		return
	}

	if authHeader := r.Header.Get("Authorization"); authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Failed to connect to ticket service at %s: %v", ticketsURL, err)
		respondWithError(w, http.StatusInternalServerError, "Ticket service unavailable")
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	w.Write(body)
}

func handleReserveTickets(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondWithError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	reserveURL := fmt.Sprintf("%s/reserve-tickets", ticketServiceURL)
	
	req, err := http.NewRequest(http.MethodPost, reserveURL, bytes.NewBuffer(body))
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to create request")
		return
	}

	req.Header.Set("Content-Type", "application/json")
	if authHeader := r.Header.Get("Authorization"); authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Failed to connect to ticket service at %s: %v", reserveURL, err)
		respondWithError(w, http.StatusInternalServerError, "Ticket service unavailable")
		return
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	w.Write(respBody)
}

func handleWebhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	webhookURL := fmt.Sprintf("%s%s", ticketServiceURL, r.URL.Path)
	
	req, err := http.NewRequest(r.Method, webhookURL, bytes.NewBuffer(body))
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to create request")
		return
	}

	req.Header.Set("Content-Type", r.Header.Get("Content-Type"))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Failed to connect to ticket service at %s: %v", webhookURL, err)
		respondWithError(w, http.StatusInternalServerError, "Ticket service unavailable")
		return
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	w.Write(respBody)
}

func handleProfile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondWithError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	profileURL := fmt.Sprintf("%s/profile?%s", dataAggregatorURL, r.URL.RawQuery)
	
	req, err := http.NewRequest(http.MethodGet, profileURL, nil)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to create request")
		return
	}

	if authHeader := r.Header.Get("Authorization"); authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Failed to connect to data aggregator at %s: %v", profileURL, err)
		respondWithError(w, http.StatusInternalServerError, "Data aggregator unavailable")
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	w.Write(body)
}

func handleWebSocketProxy(w http.ResponseWriter, r *http.Request) {
	target, err := url.Parse(websocketServiceURL)
	if err != nil {
		log.Printf("Failed to parse WebSocket URL: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	proxy := httputil.NewSingleHostReverseProxy(target)
	
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.URL.Path = r.URL.Path
		req.URL.RawQuery = r.URL.RawQuery
		req.Host = target.Host
		
		if cookie := r.Header.Get("Cookie"); cookie != "" {
			req.Header.Set("Cookie", cookie)
		}
	}

	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		log.Printf("WebSocket proxy error: %v", err)
		http.Error(w, "WebSocket connection failed", http.StatusBadGateway)
	}

	log.Printf("Proxying WebSocket connection to %s%s", target.Host, r.URL.Path)
	proxy.ServeHTTP(w, r)
}