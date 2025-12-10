package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
)

var (
	db        *sql.DB
)

var ErrUserNotFound = errors.New("user not found")


func getJWTSecret() []byte {
	secret := os.Getenv("JWT_SECRET")
	return []byte(secret)
}

func setupDatabase() error {
	dbHost := os.Getenv("DB_HOST")
		dbUser := os.Getenv("DB_USER")
		dbPassword := os.Getenv("DB_PASSWORD")
		dbName := os.Getenv("DB_NAME")
		dbPort := os.Getenv("DB_PORT")

		log.Printf("Connecting to DB: host=%s port=%s user=%s dbname=%s", dbHost, dbPort, dbUser, dbName)

		connectionString := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable", dbHost, dbPort, dbUser, dbPassword, dbName)

		var err error
		db, err = sql.Open("postgres", connectionString)
		if err != nil {
			log.Fatal("Failed to connect to database:", err)
		}

		// Test the connection
		err = db.Ping()
		if err != nil {
			log.Fatal("Failed to ping database:", err)
		}

		return err
}

func main() {
	godotenv.Load()

	err := setupDatabase()
	if err != nil {
		log.Fatal("Database setup failed:", err)
	}

	defer db.Close()

	http.HandleFunc("/register", handleRegister)
	http.HandleFunc("/login", handleLogin)

	serverPort := os.Getenv("SERVER_PORT")
	log.Printf("Starting server on port %s", serverPort)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%s", serverPort), nil))
}

func handleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var user User
	err := json.NewDecoder(r.Body).Decode(&user)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	userExists, err := userExists(user.Username)
	if err != nil || userExists {
		respondWithError(w, http.StatusBadRequest, "Username already taken")
		return
	}

	if user.Username == "" || user.Password == "" {
		respondWithError(w, http.StatusBadRequest, "Username and password are required")
		return
	}

	hashedPassword, salt, err := generateHashedPassword(user.Password)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to hash password")
		return
	}

	userID, err := insertUserInDatabase(user.Username, hashedPassword, salt)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}
	
	respondWithJSON(w, http.StatusCreated, map[string]any{
		"message": "User registered successfully",
		"user_id": userID,
	})
}

func handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var credentials User
	err := json.NewDecoder(r.Body).Decode(&credentials)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	user, err := getUserByUsername(credentials.Username)
	if err != nil {
		respondWithError(w, http.StatusUnprocessableEntity, "User does not exist")
		return
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.Password+user.Salt), []byte(credentials.Password+user.Salt))
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Invalid username or password")
		return
	}

	token, err := generateJWT(user.Username, user.ID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to generate token")
		return
	}

	respondWithJSON(w, http.StatusOK, LoginResponse{
		Token:   token,
		Message: "Login successful",
	})
}

func generateJWT(username string, userID int) (string, error) {
	expirationTime := time.Now().Add(24 * time.Hour)

	claims := &UserClaims{
		Username: username,
		UserID:   userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(getJWTSecret())
}

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	response, _ := json.Marshal(payload)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(response)
}

func respondWithError(w http.ResponseWriter, code int, message string) {
	respondWithJSON(w, code, ErrorResponse{Error: message})
}

func generateHashedPassword(password string) (hashedPassword string, salt string, err error) {
	salt = generateSalt(16)
	hashedPasswordBytes, err := bcrypt.GenerateFromPassword([]byte(password+salt), bcrypt.DefaultCost)

	if err != nil {
		return "", "", err
	}

	return string(hashedPasswordBytes), salt, nil
}

func generateSalt(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	salt := make([]byte, length)

	for i := range salt {
		num, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		salt[i] = charset[num.Int64()]
	}

	return string(salt)
}