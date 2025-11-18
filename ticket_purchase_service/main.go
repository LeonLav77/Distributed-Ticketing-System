package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
	"github.com/streadway/amqp"
	clientv3 "go.etcd.io/etcd/client/v3"
)

var (
	etcdClient      *clientv3.Client
	redisClient     *redis.Client
	rabbitMQConn    *amqp.Connection
	rabbitMQChannel *amqp.Channel
)

func handler(w http.ResponseWriter, r *http.Request) {
	port := os.Getenv("EXTERNAL_PORT")
	if port == "" {
		port = "unknown"
	}
	fmt.Fprintf(w, "Hello, World From Go on port %s!", port)
}

func healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "OK")
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func initEtcd() {
	var err error

	endpoints := []string{
		getEnv("ETCD_ENDPOINT_1", "localhost:2379"),
		getEnv("ETCD_ENDPOINT_2", "localhost:2479"),
		getEnv("ETCD_ENDPOINT_3", "localhost:2579"),
	}

	dialTimeout := getEnv("ETCD_DIAL_TIMEOUT", "5s")
	duration, err := time.ParseDuration(dialTimeout)
	if err != nil {
		duration = 5 * time.Second
	}

	etcdClient, err = clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: duration,
	})
	if err != nil {
		log.Fatalf("Failed to connect to etcd: %v", err)
	}

	log.Printf("Connected to etcd cluster at: %s", strings.Join(endpoints, ", "))
}

func initRedis() {
	redisAddr := getEnv("REDIS_ADDR", "localhost:6379")

	redisClient = redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := redisClient.Ping(ctx).Err(); err != nil {
		log.Fatalf("Failed to connect to Redis at %s: %v", redisAddr, err)
	}

	log.Printf("Connected to Redis at: %s", redisAddr)
}

func withCORS(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		corsOrigin := getEnv("CORS_ALLOWED_ORIGIN", "http://127.0.0.1:8080")

		w.Header().Set("Access-Control-Allow-Origin", corsOrigin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		h.ServeHTTP(w, r)
	})
}

func initRabbitMQ() {
	var err error

	rabbitMQURL := getEnv("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/")

	rabbitMQConn, err = amqp.Dial(rabbitMQURL)
	if err != nil {
		log.Fatalf("Failed to connect to RabbitMQ: %v", err)
	}

	rabbitMQChannel, err = rabbitMQConn.Channel()
	if err != nil {
		log.Fatalf("Failed to open a channel: %v", err)
	}

	log.Printf("Connected to RabbitMQ at: %s", rabbitMQURL)
}

func sendRabbitMQMessage(queueName string, messageBody []byte) error {
	return rabbitMQChannel.Publish("", queueName, false, false, amqp.Publishing{
		ContentType: "application/json",
		Body:        messageBody,
	})
}

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: No .env file found, using system environment variables")
	}

	log.Printf("Configuration loaded:")
	log.Printf("  REDIS_ADDR: %s", getEnv("REDIS_ADDR", "localhost:6379"))
	log.Printf("  SERVER_PORT: %s", getEnv("SERVER_PORT", "10000"))

	initRedis()
	defer redisClient.Close()

	initEtcd()
	defer etcdClient.Close()

	initRabbitMQ()
	defer rabbitMQConn.Close()
	defer rabbitMQChannel.Close()

	paymentProcessorURL := getEnv("PAYMENT_PROCESSOR_URL", "http://localhost:12222")
	log.Printf("Using payment processor at %s", paymentProcessorURL)

	http.Handle("/reserve-tickets", withCORS(authenticateJWT(handleReserveTicketsAndRedirectToCheckout)))

	http.Handle("/get-available-tickets", withCORS(http.HandlerFunc(handleGetAvailableTickets)))
	http.Handle("/health-check", withCORS(http.HandlerFunc(healthCheckHandler)))
	http.Handle("/webhooks/payment-success", withCORS(http.HandlerFunc(handlePaymentSuccess)))
	http.Handle("/webhooks/payment-cancel", withCORS(http.HandlerFunc(handlePaymentCancel)))
	http.Handle("/", withCORS(http.HandlerFunc(handler)))

	serverPort := getEnv("SERVER_PORT", "10000")
	fmt.Printf("Server starting on port %s...\n", serverPort)
	log.Fatal(http.ListenAndServe(":"+serverPort, nil))
}

func handleReserveTicketsAndRedirectToCheckout(w http.ResponseWriter, r *http.Request) {
	var requestData ReserveTicketsRequest

	if err := json.NewDecoder(r.Body).Decode(&requestData); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	log.Printf("Received reservation request: %+v", requestData)

	success, err := checkIfTicketsAvailableAndReserve(requestData.EventId, requestData.TicketType, requestData.Quantity)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if !success {
		http.Error(w, "Not enough tickets available", http.StatusBadRequest)
		return
	}

	userJWT := r.Header.Get("Authorization")

	// Create checkout session with payment processor
	userID := extractUserIDFromJWT(userJWT)
	orderReferenceId := generateOrderReferenceID()

	go sendRabbitMQMessage("order.created", []byte(fmt.Sprintf(`{"event_id":"%s","ticket_type":"%s","quantity":%d,"user_id":%d,"order_reference_id":"%s"}`, requestData.EventId, requestData.TicketType, requestData.Quantity, userID, orderReferenceId)))

	checkoutURL, err := createCheckoutSession(orderReferenceId, requestData)
	if err != nil {
		log.Printf("Failed to create checkout session: %v", err)
		http.Error(w, "Failed to create checkout session", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(TicketReservationResponse{
		Success:     true,
		CheckoutURL: checkoutURL,
	})
}

func handleGetAvailableTickets(w http.ResponseWriter, r *http.Request) {
	eventId := r.URL.Query().Get("eventId")
	admissionToken := r.URL.Query().Get("admission_token")
	if eventId == "" || admissionToken == "" {
		http.Error(w, "Missing eventId or admission_token parameter", http.StatusBadRequest)
		return
	}

	if !validateAdmissionToken(eventId, admissionToken) {
		http.Error(w, "Invalid admission token", http.StatusUnauthorized)
		return
	}

	ticketType := "basic"
	key := fmt.Sprintf("event:%s:available_tickets:%s", eventId, ticketType)

	availableBasicTickets, err := redisClient.Get(r.Context(), key).Result()

	var quantity int
	if err == redis.Nil || err != nil {
		log.Printf("Redis miss for eventId %s, fetching from etcd", eventId)
		quantity, etcdErr := getAvailableTicketsFromEtcd(eventId, ticketType)

		if etcdErr != nil {
			log.Printf("Error fetching from etcd: %v", etcdErr)
			http.Error(w, "Error fetching tickets", http.StatusInternalServerError)
			return
		}

		log.Printf("Fetched from etcd: %d basic tickets available", quantity)
		go storeAvailableTicketsInRedis(eventId, ticketType, quantity)
	} else {
		quantity, _ = strconv.Atoi(availableBasicTickets)
	}

	response := TicketAvailabilityResponse{
		EventId: eventId,
		AvailableTickets: []TicketAvailabilityEntry{
			{TicketType: ticketType, Quantity: quantity},
		},
	}

	log.Printf("Returning tickets: %+v", response.AvailableTickets)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func storeAvailableTicketsInRedis(eventId string, ticketType string, quantity int) error {
	key := fmt.Sprintf("event:%s:available_tickets:%s", eventId, ticketType)
	ctx := context.Background()

	cacheTTL := getEnv("REDIS_CACHE_TTL", "10s")
	duration, err := time.ParseDuration(cacheTTL)
	if err != nil {
		duration = 10 * time.Second
	}

	return redisClient.Set(ctx, key, quantity, duration).Err()
}

func getAvailableTicketsFromEtcd(eventId string, tier string) (int, error) {
	ctx := context.Background()
	key := fmt.Sprintf("concert:%s:available:%s", eventId, tier)

	resp, err := etcdClient.Get(ctx, key)
	if err != nil || len(resp.Kvs) == 0 {
		return 0, err
	}

	available, _ := strconv.Atoi(string(resp.Kvs[0].Value))

	return available, nil
}

func checkIfTicketsAvailableAndReserve(eventId string, tier string, requestAmount int) (bool, error) {
	maxRetriesStr := getEnv("MAX_RETRIES", "10")
	maxRetries, err := strconv.Atoi(maxRetriesStr)
	if err != nil {
		maxRetries = 10
	}

	retryDelayStr := getEnv("RETRY_DELAY", "10ms")
	retryDelay, err := time.ParseDuration(retryDelayStr)
	if err != nil {
		retryDelay = 10 * time.Millisecond
	}

	reservationTimeoutStr := getEnv("RESERVATION_TIMEOUT", "15m")
	reservationTimeout, err := time.ParseDuration(reservationTimeoutStr)
	if err != nil {
		reservationTimeout = 15 * time.Minute
	}

	for range maxRetries {
		ctx := context.Background()
		key := fmt.Sprintf("concert:%s:available:%s", eventId, tier)

		availableTickets, err := etcdClient.Get(ctx, key)
		if err != nil {
			return false, err
		}

		if len(availableTickets.Kvs) == 0 {
			return false, fmt.Errorf("concert not found")
		}

		currentValue := string(availableTickets.Kvs[0].Value)
		currentInt, _ := strconv.Atoi(currentValue)

		if currentInt < requestAmount {
			return false, fmt.Errorf("not enough tickets")
		}

		newValue := currentInt - requestAmount

		transactionResponse, err := etcdClient.Txn(ctx).
			If(clientv3.Compare(clientv3.Version(key), "=", availableTickets.Kvs[0].Version)).
			Then(clientv3.OpPut(key, strconv.Itoa(newValue))).
			Commit()

		if err != nil {
			return false, err
		}

		if transactionResponse.Succeeded {
			reservationExpiry := time.Now().Add(reservationTimeout)
			reservationKey := fmt.Sprintf("reservation:%s:%s:%s", eventId, tier, reservationExpiry.Format(time.RFC3339))

			etcdClient.Put(ctx, reservationKey, strconv.Itoa(requestAmount))

			go storeAvailableTicketsInRedis(eventId, tier, newValue)

			return true, nil
		}

		time.Sleep(retryDelay)
	}

	return false, fmt.Errorf("too many retries")
}

func generateOrderReferenceID() string {
	return fmt.Sprintf("%d-%s", time.Now().UnixMilli(), randomString(20))
}

func randomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

	seededRand := rand.New(rand.NewSource(time.Now().UnixNano()))
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}
	return string(b)
}

func validateAdmissionToken(eventId string, token string) bool {
	claims := AdmissionsClaims{}
	parsedToken, err := jwt.ParseWithClaims(token, &claims, func(token *jwt.Token) (any, error) {
		return getJWTSecret(), nil
	})

	if err != nil || parsedToken == nil || !parsedToken.Valid {
		log.Printf("Admission token validation failed - error: %v, eventId attempted: %s", err, claims.EventID)
		return false
	}
	if claims.EventID != eventId {
		log.Printf("Admission token event ID mismatch - token eventId: %s, requested eventId: %s", claims.EventID, eventId)
		return false
	}

	return true
}