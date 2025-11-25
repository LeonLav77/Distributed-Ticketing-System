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
	"strings"
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

func handleAPIProxy(ticketServiceURL, dataAggregatorURL string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/login") || strings.HasPrefix(r.URL.Path, "/api/register") {
			return
		}

		var targetURL string
		
		if strings.HasPrefix(r.URL.Path, "/api/get-available-tickets") || 
		   strings.HasPrefix(r.URL.Path, "/api/reserve-tickets") ||
		   strings.HasPrefix(r.URL.Path, "/api/webhooks/") {
			targetURL = ticketServiceURL
		} else if strings.HasPrefix(r.URL.Path, "/api/profile") {
			targetURL = dataAggregatorURL
		} else {
			http.NotFound(w, r)
			return
		}

		target, err := url.Parse(targetURL)
		if err != nil {
			log.Printf("Failed to parse API URL: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		proxy := httputil.NewSingleHostReverseProxy(target)

		originalDirector := proxy.Director
		proxy.Director = func(req *http.Request) {
			originalDirector(req)
			req.URL.Path = strings.TrimPrefix(r.URL.Path, "/api")
			req.URL.RawQuery = r.URL.RawQuery
			req.Host = target.Host
			
			if authHeader := r.Header.Get("Authorization"); authHeader != "" {
				req.Header.Set("Authorization", authHeader)
			}
			
			if cookie := r.Header.Get("Cookie"); cookie != "" {
				req.Header.Set("Cookie", cookie)
			}
		}

		proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
			log.Printf("API proxy error for %s: %v", r.URL.Path, err)
			http.Error(w, "API request failed", http.StatusBadGateway)
		}

		log.Printf("Proxying API request %s to %s%s", r.URL.Path, target.Host, strings.TrimPrefix(r.URL.Path, "/api"))
		proxy.ServeHTTP(w, r)
	}
}

func handleWebSocketProxy(websocketServiceURL string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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
}