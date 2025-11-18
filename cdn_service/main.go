package main

import (
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
)

func getPageData() PageData {
	return PageData{
		APIBaseURL:     os.Getenv("API_BASE_URL"),
		AuthServiceURL: os.Getenv("AUTH_SERVICE_URL"),
		FrontendURL:    os.Getenv("FRONTEND_URL"),
		DirectusAPIURL: os.Getenv("DIRECTUS_API_URL"),
		WebsocketURL:   os.Getenv("WEBSOCKET_URL"),
	}
}

func main() {
	godotenv.Load()

	authServiceURL := os.Getenv("AUTH_SERVICE_URL")

	staticFilesPath := os.Getenv("STATIC_FILES_PATH")
	serverPort := os.Getenv("SERVER_PORT")

	log.Printf("Configuration loaded:")
	log.Printf("  Auth Service: %s", authServiceURL)
	log.Printf("  API Base URL: %s", os.Getenv("API_BASE_URL"))
	log.Printf("  Static Files: %s", staticFilesPath)
	log.Printf("  Server Port: %s", serverPort)

	// Auth endpoints
	http.HandleFunc("/api/login", handleLoginAPI(authServiceURL))
	http.HandleFunc("/api/register", handleRegisterAPI(authServiceURL))
	http.HandleFunc("/login", handleLoginPage(staticFilesPath))
	http.HandleFunc("/register", handleRegisterPage(staticFilesPath))

	// Protected routes
	http.HandleFunc("/queue", authenticateJWT(handleQueue(staticFilesPath)))
	http.HandleFunc("/profile", authenticateJWT(handleProfile(staticFilesPath)))
	http.HandleFunc("/choose-tickets", authenticateJWT(handleChooseTickets(staticFilesPath)))

	// Public routes
	http.HandleFunc("/", handleIndex(staticFilesPath))

	log.Printf("🚀 Frontend server starting on port %s...", serverPort)
	log.Fatal(http.ListenAndServe(":"+serverPort, nil))
}

func handleLoginPage(staticFilesPath string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		filePath := filepath.Join(staticFilesPath, "login.html")
		template, err := template.ParseFiles(filePath)

		if err != nil {
			http.Error(w, "Template error", http.StatusInternalServerError)
			return
		}
		template.Execute(w, getPageData())
	}
}

func handleRegisterPage(staticFilesPath string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		filePath := filepath.Join(staticFilesPath, "register.html")
		template, err := template.ParseFiles(filePath)

		if err != nil {
			http.Error(w, "Template error", http.StatusInternalServerError)
			return
		}
		template.Execute(w, getPageData())
	}
}

func handleQueue(staticFilesPath string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		eventId := r.URL.Query().Get("event_id")
		if eventId == "" {
			http.Error(w, "event_id parameter is required", http.StatusBadRequest)
			return
		}
		filePath := filepath.Join(staticFilesPath, "queue.html")
		tmpl, err := template.ParseFiles(filePath)
		if err != nil {
			http.Error(w, "Template error", http.StatusInternalServerError)
			return
		}
		tmpl.Execute(w, getPageData())
	}
}

func handleProfile(staticFilesPath string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		filePath := filepath.Join(staticFilesPath, "profile.html")
		tmpl, err := template.ParseFiles(filePath)
		
		if err != nil {
			http.Error(w, "Template error", http.StatusInternalServerError)
			return
		}
		tmpl.Execute(w, getPageData())
	}
}

func handleChooseTickets(staticFilesPath string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		filePath := filepath.Join(staticFilesPath, "choose-tickets.html")
		tmpl, err := template.ParseFiles(filePath)

		if err != nil {
			http.Error(w, "Template error", http.StatusInternalServerError)
			return
		}
		tmpl.Execute(w, getPageData())
	}
}

func handleIndex(staticFilesPath string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		filePath := filepath.Join(staticFilesPath, "index.html")
		tmpl, err := template.ParseFiles(filePath)

		if err != nil {
			http.Error(w, "Template error", http.StatusInternalServerError)
			return
		}
		tmpl.Execute(w, getPageData())
	}
}
