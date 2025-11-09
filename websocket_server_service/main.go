package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"time"

	"github.com/google/uuid"
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
	clientID string
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
	redisHost := os.Getenv("REDIS_HOST")
	redisPort := os.Getenv("REDIS_PORT")

	redisAddr := redisHost + ":" + redisPort

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
	router.HandleFunc("/ws", handleWebSocket)
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

	// TODO: JE LI BOLJE OVO ILI REMOTE ADDR
	clientID := uuid.New().String()

	client := &Client{
		conn:     conn,
		clientID: clientID,
		eventID:  eventID,
	}

	log.Printf("Generated clientID: %s for eventId: %s", clientID, eventID)

	queueKey := "ws-queue:" + eventID
	err := addToQueue(queueKey, client.clientID)
	if err != nil {
		log.Printf("Error adding client to queue: %v", err)
		return
	}
	defer removeFromQueue(queueKey, client.clientID)

	done := make(chan struct{})

	go sendQueueUpdates(client, queueKey, done)
	awaitMessages(client, done)

	log.Printf("WebSocket connection closed - eventId: %s, clientId: %s", eventID, clientID)
}

func addToQueue(queueKey, clientID string) error {
	// koristi se timestamp kao score jer on osigurava redoslijed
	score := float64(time.Now().Unix())
	err := rdb.ZAdd(ctx, queueKey, redis.Z{
		Score:  score,
		Member: clientID,
	}).Err()

	if err != nil {
		return err
	}

	log.Printf("Added %s to queue %s", clientID, queueKey)
	return nil
}

func removeFromQueue(queueKey, clientID string) {
	err := rdb.ZRem(ctx, queueKey, clientID).Err()
	if err != nil {
		log.Printf("Error removing %s from queue: %v", clientID, err)
	} else {
		log.Printf("Removed %s from queue %s", clientID, queueKey)
	}
}

func getQueuePosition(queueKey, clientID string) (int, error) {
	rank, err := rdb.ZRank(ctx, queueKey, clientID).Result()
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
	position, err := getQueuePosition(queueKey, client.clientID)
	if err != nil {
		log.Printf("Error getting queue position for %s: %v", client.clientID, err)
		return false, err
	}

	totalInQueue, err := getQueueSize(queueKey)
	if err != nil {
		log.Printf("Error getting queue size: %v", err)
		return false, err
	}

	acceptedTokenId := ""
	shouldClose := false
	if position < 50 {
		acceptedTokenId = generateToken()
		shouldClose = true
	}

	updateMessage := NewQueueUpdateMessage{
		EstimatedWaitTime:  (position + 1) * 30,
		PositionInQueue:    position + 1,
		PeopleAheadInQueue: position,
		TotalPeopleInQueue: totalInQueue,
		AcceptedTokenId:    acceptedTokenId,
	}

	log.Printf("About to send message to %s: %+v", client.clientID, updateMessage)

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
		log.Printf("Client %s accepted with token, closing connection", client.clientID)

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
				log.Printf("Client %s accepted with token, closing connection", client.clientID)
				client.conn.WriteControl(websocket.CloseMessage,
					websocket.FormatCloseMessage(websocket.CloseNormalClosure, "accepted"),
					time.Now().Add(time.Second))
				return
			}

		case <-done:
			log.Printf("Received done signal for client %s", client.clientID)
			return
		}
	}
}

func generateToken() string {
	return uuid.New().String()
}

func awaitMessages(client *Client, done chan struct{}) {
	defer close(done)

	for {
		var msg map[string]any
		err := client.conn.ReadJSON(&msg)

		if err != nil {
			log.Printf("Error reading message from clientId %s: %v", client.clientID, err)
			break
		}
		log.Printf("Received message from clientId %s: %v", client.clientID, msg)

		// Echo back
		err = client.conn.WriteJSON(msg)
		if err != nil {
			log.Printf("Error writing message to clientId %s: %v", client.clientID, err)
			break
		}
	}
}