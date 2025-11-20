package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"

	"github.com/joho/godotenv"
)

var (
	ctx = context.Background()
	rdb *redis.Client
)

type Client struct {
	conn     *websocket.Conn
	userID   int
	eventID  string
}

type NewQueueUpdateMessage struct {
	EstimatedWaitTime  int    `json:"estimatedWaitTime"`
	PositionInQueue    int    `json:"positionInQueue"`
	PeopleAheadInQueue int    `json:"peopleAheadInQueue"`
	TotalPeopleInQueue int    `json:"totalPeopleInQueue"`
	AcceptedTokenId    string `json:"acceptedTokenId"`
}

func initRedis() {
	redisAddr := os.Getenv("REDIS_ADDR")

	rdb = redis.NewClient(&redis.Options{
		Addr:            redisAddr,
		Password:        "", // no password set
		DB:              0,  // use default DB
		DisableIdentity: true,
		PoolSize:        1000, // Increase pool size for many connections
		MinIdleConns:    100,
	})
}

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU() - 2) // use almost all CPUs

	godotenv.Load()

	initRedis()

	router := mux.NewRouter()
	router.HandleFunc("/ws", authenticateJWT(handleWebSocket))
	http.Handle("/", router)

	serverPort := os.Getenv("SERVER_PORT")

	log.Printf("WebSocket server started on :%s", serverPort)
	log.Fatal(http.ListenAndServe(":"+serverPort, router))
}

func upgradeConnection(w http.ResponseWriter, r *http.Request) *websocket.Conn {
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
		ReadBufferSize:  4096,
		WriteBufferSize: 4096,
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Error upgrading connection:", err)
		return nil
	}

	return conn
}

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	eventID := r.URL.Query().Get("eventId")
	if eventID == "" {
		http.Error(w, "eventId parameter is required", http.StatusBadRequest)
		return
	}

	log.Printf("New WebSocket connection - eventId: %s", eventID)

	conn := upgradeConnection(w, r)
	if conn == nil {
		return
	}
	defer conn.Close()

	cookie, err := r.Cookie("auth_token")
	if err != nil {
		log.Printf("No auth_token cookie found for userID")
	}
	tokenString := cookie.Value
	userID := extractUserIDFromJWT(tokenString)

	client := &Client{
		conn:     conn,
		userID:   userID,
		eventID:  eventID,
	}

	log.Printf("Generated userID: %d for eventId: %s", userID, eventID)
	queueKey := "ws-queue:" + eventID
	err = addToQueue(queueKey, userID)
	if err != nil {
		log.Printf("Error adding client to queue: %v", err)
		return
	}
	defer removeFromQueue(queueKey, userID)

	done := make(chan struct{})

	go sendQueueUpdates(client, queueKey, done)
	awaitMessages(client, done)

	log.Printf("WebSocket connection closed - eventId: %s, userId: %d", eventID, userID)
}

func addToQueue(queueKey string, userID int) error {
	// koristi se timestamp kao score jer on osigurava redoslijed
	score := float64(time.Now().Unix())
	err := rdb.ZAdd(ctx, queueKey, redis.Z{
		Score:  score,
		Member: userID,
	}).Err()

	if err != nil {
		return err
	}

	log.Printf("Added %d to queue %s", userID, queueKey)
	return nil
}

func removeFromQueue(queueKey string, userID int) {
	err := rdb.ZRem(ctx, queueKey, userID).Err()
	if err != nil {
		log.Printf("Error removing %d from queue: %v", userID, err)
	} else {
		log.Printf("Removed %d from queue %s", userID, queueKey)
	}
}

func getQueuePosition(queueKey string, userID int) (int, error) {
	rank, err := rdb.ZRank(ctx, queueKey, strconv.Itoa(userID)).Result()
	if err != nil {
		return -1, err
	}
	return int(rank), nil
}

func getQueueSize(queueKey string) (int, error) {
	size, err := rdb.ZCard(ctx, queueKey).Result()
	return int(size), err
}

func sendUpdate(client *Client, queueKey string) (bool, error) {
	position, err := getQueuePosition(queueKey, client.userID)
	if err != nil {
		log.Printf("Error getting queue position for %d: %v", client.userID, err)
		return false, err
	}

	totalInQueue, err := getQueueSize(queueKey)
	if err != nil {
		log.Printf("Error getting queue size: %v", err)
		return false, err
	}

	acceptedTokenId := ""
	shouldClose := false
	if position < 1 {
		acceptedTokenId, err = generateAdmissionJWT(client.userID, client.eventID)
		if err != nil {
			log.Printf("Error generating admission JWT for user %d: %v", client.userID, err)
			return false, err
		}
		shouldClose = true
	}

	updateMessage := NewQueueUpdateMessage{
		EstimatedWaitTime:  (position + 1) * 30,
		PositionInQueue:    position + 1,
		PeopleAheadInQueue: position,
		TotalPeopleInQueue: totalInQueue,
		AcceptedTokenId:    acceptedTokenId,
	}

	log.Printf("About to send message to %d: %+v", client.userID, updateMessage)

	err = client.conn.WriteJSON(updateMessage)
	if err != nil {
		return false, err
	}

	return shouldClose, nil
}

func sendQueueUpdates(client *Client, queueKey string, done chan struct{}) {
	shouldClose, err := sendUpdate(client, queueKey)

	if err != nil {
		log.Printf("Failed to send initial update, closing connection")
		return
	} else if shouldClose {
		log.Printf("Client %d accepted with token, closing connection", client.userID)

		// Send close message (polite notification)
		client.conn.WriteControl(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, "accepted"),
			time.Now().Add(time.Second))

		// FORCE close the underlying connection - don't wait for client
		client.conn.Close()
		return
	}

	// send update every x seconds
	updateTimeMsStr, _ :=  strconv.ParseInt(os.Getenv("WEBSOCKET_UPDATE_TIME_MS"), 10, 64)
	ticker := time.NewTicker(time.Duration(updateTimeMsStr) * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			shouldClose, err := sendUpdate(client, queueKey)

			if err != nil {
				return
			} else if shouldClose {
				log.Printf("Client %d accepted with token, closing connection", client.userID)
				client.conn.WriteControl(websocket.CloseMessage,
					websocket.FormatCloseMessage(websocket.CloseNormalClosure, "accepted"),
					time.Now().Add(time.Second))
				return
			}

		case <-done:
			log.Printf("Received done signal for client %d", client.userID)
			return
		}
	}
}

func generateAdmissionJWT(userID int, eventID string) (string, error) {
	expirationTime := time.Now().Add(24 * time.Hour)
	
	claims := &AdmissionsClaims{
		EventID: eventID,
		UserID:   userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(getJWTSecret())
}

func awaitMessages(client *Client, done chan struct{}) {
	defer close(done)

	for {
		var msg map[string]any
		err := client.conn.ReadJSON(&msg)

		if err != nil {
			log.Printf("Error reading message from userId %d: %v", client.userID, err)
			break
		}
		log.Printf("Received message from userId %d: %v", client.userID, msg)

		// Echo back
		err = client.conn.WriteJSON(msg)
		if err != nil {
			log.Printf("Error writing message to userId %d: %v", client.userID, err)
			break
		}
	}
}