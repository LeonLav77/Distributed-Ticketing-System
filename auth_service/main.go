package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/json"
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

func main() {
	godotenv.Load()

    dbUrl := os.Getenv("DB_URL")
	dbUser := os.Getenv("DB_USER")
	dbPassword := os.Getenv("DB_PASSWORD")
	dbName := os.Getenv("DB_NAME")
	dbPort := os.Getenv("DB_PORT")

	connectionString := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable", dbUrl, dbPort, dbUser, dbPassword, dbName)

	var err error
	db, err = sql.Open("postgres", connectionString)
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}
	defer db.Close()

	http.HandleFunc("/register", handleRegister)
	http.HandleFunc("/login", handleLogin)

	serverPort := os.Getenv("SERVER_PORT")
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

	// Hash password
	hashedPassword, salt, err := generateHashedPassword(user.Password)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to hash password")
		return
	}

	// Insert user into database
	userID, err := insertUserInDatabase(user.Username, hashedPassword, salt)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}
	log.Printf("User registered successfully: %d\n", userID)
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
	
	claims := &Claims{
		Username: username,
		UserID:   userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret)
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