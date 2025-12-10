package main

import (
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
)

func getPageData() PageData {
	return PageData{
		APIBaseURL:     "/api",
		AuthServiceURL: os.Getenv("AUTH_SERVICE_URL"),
		DirectusAPIURL: os.Getenv("DIRECTUS_API_URL"),
		WebsocketURL:   "/ws",
	}
}

func renderPage(filename string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		filePath := filepath.Join(staticFilesPath, filename)
		tmpl, err := template.ParseFiles(filePath)
		if err != nil {
			http.Error(w, "Template error", http.StatusInternalServerError)
			return
		}
		tmpl.Execute(w, getPageData())
	}
}

var (
	staticFilesPath string
	authServiceURL  string
	ticketServiceURL string
	dataAggregatorURL string
	websocketServiceURL string
)

func main() {
	godotenv.Load()

	authServiceURL = os.Getenv("AUTH_SERVICE_URL")
	ticketServiceURL = os.Getenv("TICKET_SERVICE_URL")
	dataAggregatorURL = os.Getenv("DATA_AGGREGATOR_URL")
	websocketServiceURL = os.Getenv("WEBSOCKET_SERVICE_URL")
	staticFilesPath = os.Getenv("STATIC_FILES_PATH")
	serverPort := os.Getenv("SERVER_PORT")

	// API endpoints
	http.HandleFunc("/api/login", handleLoginAPI)
	http.HandleFunc("/api/register", handleRegisterAPI)
	http.HandleFunc("/api/get-available-tickets", handleGetAvailableTickets)
	http.HandleFunc("/api/reserve-tickets", handleReserveTickets)
	http.HandleFunc("/api/webhooks/", handleWebhook)
	http.HandleFunc("/api/profile", handleProfile)
	http.HandleFunc("/ws", handleWebSocketProxy)

	// Auth pages
	http.HandleFunc("/login", renderPage("login.html"))
	http.HandleFunc("/register", renderPage("register.html"))

	// Protected routes
	http.HandleFunc("/queue", authenticateJWT(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("event_id") == "" {
			http.Error(w, "event_id parameter is required", http.StatusBadRequest)
			return
		}
		renderPage("queue.html")(w, r)
	}))
	http.HandleFunc("/profile", authenticateJWT(renderPage("profile.html")))
	http.HandleFunc("/choose-tickets", authenticateJWT(renderPage("choose-tickets.html")))

	// Public routes
	http.HandleFunc("/", renderPage("index.html"))

	log.Fatal(http.ListenAndServe(":"+serverPort, nil))
}

func respondWithError(w http.ResponseWriter, code int, message string) {
	respondWithJSON(w, code, ErrorResponse{Error: message})
}

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	response, _ := json.Marshal(payload)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(response)
}