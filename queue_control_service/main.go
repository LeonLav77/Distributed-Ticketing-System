package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
)

var (
	ctx      = context.Background()
	rdb      *redis.Client
	upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}
)

func initRedis() {
	redisAddr := os.Getenv("REDIS_ADDR")

	rdb = redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: "",
		DB:       0,
	})

	_, err := rdb.Ping(ctx).Result()
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	log.Println("Connected to Redis")
}

func getQueueState(eventID string) (*QueueState, error) {
	queueKey := "ws-queue:" + eventID

	members, err := rdb.ZRangeWithScores(ctx, queueKey, 0, -1).Result()
	if err != nil {
		return nil, err
	}

	userIDs := make([]string, len(members))
	for i, member := range members {
		userIDs[i] = fmt.Sprintf("%v", member.Member)
	}

	return &QueueState{
		EventID:      eventID,
		TotalInQueue: len(members),
		UserIDs:      userIDs,
	}, nil
}

func addConnections(eventID string, count int) error {
	queueKey := "ws-queue:" + eventID

	for i := 0; i < count; i++ {
		userID := time.Now().UnixNano()
		score := float64(time.Now().Unix())

		err := rdb.ZAdd(ctx, queueKey, redis.Z{
			Score:  score,
			Member: userID,
		}).Err()

		if err != nil {
			return err
		}
	}

	log.Printf("Added %d connections to queue %s", count, queueKey)
	return nil
}

func removeConnections(eventID string, count int) error {
	queueKey := "ws-queue:" + eventID

	members, err := rdb.ZRange(ctx, queueKey, 0, int64(count-1)).Result()
	if err != nil {
		return err
	}

	if len(members) == 0 {
		return nil
	}

	toRemove := make([]any, len(members))
	for i, member := range members {
		toRemove[i] = member
	}

	err = rdb.ZRem(ctx, queueKey, toRemove...).Err()
	if err != nil {
		return err
	}

	log.Printf("Removed %d connections from queue %s", len(members), queueKey)
	return nil
}

func clearQueue(eventID string) error {
	queueKey := "ws-queue:" + eventID
	err := rdb.Del(ctx, queueKey).Err()
	if err != nil {
		return err
	}
	log.Printf("Cleared queue %s", queueKey)
	return nil
}

func handleGetQueueState(w http.ResponseWriter, r *http.Request) {
	eventID := r.URL.Query().Get("eventId")
	if eventID == "" {
		http.Error(w, "eventId parameter is required", http.StatusBadRequest)
		return
	}

	state, err := getQueueState(eventID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(state)
}

func handleAddConnections(w http.ResponseWriter, r *http.Request) {
	var req AddConnectionsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.EventID == "" || req.Count <= 0 {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	err := addConnections(req.EventID, req.Count)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	state, _ := getQueueState(req.EventID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(state)
}

func handleRemoveConnections(w http.ResponseWriter, r *http.Request) {
	var req RemoveConnectionsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.EventID == "" || req.Count <= 0 {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	err := removeConnections(req.EventID, req.Count)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	state, _ := getQueueState(req.EventID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(state)
}

func handleClearQueue(w http.ResponseWriter, r *http.Request) {
	eventID := r.URL.Query().Get("eventId")
	if eventID == "" {
		http.Error(w, "eventId parameter is required", http.StatusBadRequest)
		return
	}

	err := clearQueue(eventID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	state, _ := getQueueState(eventID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(state)
}

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}
	defer conn.Close()

	eventID := r.URL.Query().Get("eventId")
	if eventID == "" {
		eventID = "58b85029-af94-498e-ae3a-2fda2b5d6c5a"
	}

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

    for range ticker.C {
        state, err := getQueueState(eventID)
        if err != nil {
            log.Printf("Error getting queue state: %v", err)
            return
        }

        if err := conn.WriteJSON(state); err != nil {
            return
        }
    }
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	html := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Queue Control Panel</title>
    <style>
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }
        
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, sans-serif;
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            min-height: 100vh;
            padding: 20px;
        }
        
        .container {
            max-width: 800px;
            margin: 0 auto;
            background: white;
            border-radius: 12px;
            box-shadow: 0 20px 60px rgba(0,0,0,0.3);
            padding: 30px;
        }
        
        h1 {
            color: #333;
            margin-bottom: 10px;
            font-size: 28px;
        }
        
        .subtitle {
            color: #666;
            margin-bottom: 30px;
            font-size: 14px;
        }
        
        .event-section {
            background: #f8f9fa;
            padding: 20px;
            border-radius: 8px;
            margin-bottom: 30px;
        }
        
        label {
            display: block;
            font-weight: 600;
            color: #333;
            margin-bottom: 8px;
            font-size: 14px;
        }
        
        input[type="text"], input[type="number"] {
            width: 100%;
            padding: 12px;
            border: 2px solid #e0e0e0;
            border-radius: 6px;
            font-size: 14px;
            transition: border-color 0.3s;
        }
        
        input[type="text"]:focus, input[type="number"]:focus {
            outline: none;
            border-color: #667eea;
        }
        
        .status-card {
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            color: white;
            padding: 25px;
            border-radius: 8px;
            margin-bottom: 30px;
        }
        
        .status-label {
            font-size: 14px;
            opacity: 0.9;
            margin-bottom: 5px;
        }
        
        .status-value {
            font-size: 36px;
            font-weight: bold;
        }
        
        .controls {
            display: grid;
            grid-template-columns: 1fr 1fr;
            gap: 15px;
            margin-bottom: 20px;
        }
        
        .control-group {
            display: flex;
            gap: 10px;
        }
        
        button {
            flex: 1;
            padding: 12px 20px;
            border: none;
            border-radius: 6px;
            font-size: 14px;
            font-weight: 600;
            cursor: pointer;
            transition: all 0.3s;
        }
        
        .btn-add {
            background: #10b981;
            color: white;
        }
        
        .btn-add:hover {
            background: #059669;
            transform: translateY(-2px);
            box-shadow: 0 4px 12px rgba(16, 185, 129, 0.4);
        }
        
        .btn-remove {
            background: #ef4444;
            color: white;
        }
        
        .btn-remove:hover {
            background: #dc2626;
            transform: translateY(-2px);
            box-shadow: 0 4px 12px rgba(239, 68, 68, 0.4);
        }
        
        .btn-clear {
            background: #f59e0b;
            color: white;
            grid-column: span 2;
        }
        
        .btn-clear:hover {
            background: #d97706;
            transform: translateY(-2px);
            box-shadow: 0 4px 12px rgba(245, 158, 11, 0.4);
        }
        
        .user-list {
            background: #f8f9fa;
            border-radius: 8px;
            padding: 20px;
            max-height: 300px;
            overflow-y: auto;
        }
        
        .user-list h3 {
            color: #333;
            margin-bottom: 15px;
            font-size: 16px;
        }
        
        .user-item {
            background: white;
            padding: 10px 15px;
            margin-bottom: 8px;
            border-radius: 6px;
            border-left: 3px solid #667eea;
            font-family: 'Courier New', monospace;
            font-size: 13px;
            color: #555;
        }
        
        .connection-status {
            display: inline-block;
            padding: 4px 12px;
            border-radius: 12px;
            font-size: 12px;
            font-weight: 600;
            margin-bottom: 20px;
        }
        
        .connected {
            background: #d1fae5;
            color: #065f46;
        }
        
        .disconnected {
            background: #fee2e2;
            color: #991b1b;
        }
        
        ::-webkit-scrollbar {
            width: 8px;
        }
        
        ::-webkit-scrollbar-track {
            background: #e0e0e0;
            border-radius: 4px;
        }
        
        ::-webkit-scrollbar-thumb {
            background: #667eea;
            border-radius: 4px;
        }
    </style>
</head>
<body>
    <div class="container">
        <h1>🎫 Queue Control Panel</h1>
        <p class="subtitle">Monitor and control WebSocket queue state</p>
        
        <div class="event-section">
            <label for="eventId">Event ID:</label>
            <input type="text" id="eventId" value="58b85029-af94-498e-ae3a-2fda2b5d6c5a" placeholder="Enter event ID">
        </div>
        
        <span class="connection-status disconnected" id="wsStatus">Disconnected</span>
        
        <div class="status-card">
            <div class="status-label">Total in Queue</div>
            <div class="status-value" id="queueSize">0</div>
        </div>
        
        <div class="controls">
            <div class="control-group">
                <input type="number" id="addCount" value="10" min="1" placeholder="Count">
                <button class="btn-add" onclick="addConnections()">➕ Add</button>
            </div>
            <div class="control-group">
                <input type="number" id="removeCount" value="5" min="1" placeholder="Count">
                <button class="btn-remove" onclick="removeConnections()">➖ Remove</button>
            </div>
            <button class="btn-clear" onclick="clearQueue()">🗑️ Clear All</button>
        </div>
        
        <div class="user-list">
            <h3>Users in Queue</h3>
            <div id="userList"></div>
        </div>
    </div>

    <script>
        let ws = null;
        let currentEventId = document.getElementById('eventId').value;

        function connectWebSocket() {
            const eventId = document.getElementById('eventId').value;
            currentEventId = eventId;
            
            if (ws) {
                ws.close();
            }
            
            ws = new WebSocket('ws://' + window.location.host + '/ws?eventId=' + eventId);
            
            ws.onopen = () => {
                console.log('WebSocket connected');
                document.getElementById('wsStatus').textContent = 'Connected';
                document.getElementById('wsStatus').className = 'connection-status connected';
            };
            
            ws.onclose = () => {
                console.log('WebSocket disconnected');
                document.getElementById('wsStatus').textContent = 'Disconnected';
                document.getElementById('wsStatus').className = 'connection-status disconnected';
                setTimeout(connectWebSocket, 2000);
            };
            
            ws.onmessage = (event) => {
                const data = JSON.parse(event.data);
                updateQueueState(data);
            };
        }

        function updateQueueState(state) {
            document.getElementById('queueSize').textContent = state.totalInQueue;
            
            const userList = document.getElementById('userList');
            if (state.userIds && state.userIds.length > 0) {
                userList.innerHTML = state.userIds.map((userId, index) => 
                    '<div class="user-item">#' + (index + 1) + ' - User ID: ' + userId + '</div>'
                ).join('');
            } else {
                userList.innerHTML = '<div style="text-align: center; color: #999; padding: 20px;">No users in queue</div>';
            }
        }

        async function addConnections() {
            const eventId = document.getElementById('eventId').value;
            const count = parseInt(document.getElementById('addCount').value);
            
            const response = await fetch('/api/add', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ eventId, count })
            });
            
            const state = await response.json();
            updateQueueState(state);
        }

        async function removeConnections() {
            const eventId = document.getElementById('eventId').value;
            const count = parseInt(document.getElementById('removeCount').value);
            
            const response = await fetch('/api/remove', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ eventId, count })
            });
            
            const state = await response.json();
            updateQueueState(state);
        }

        async function clearQueue() {
            const eventId = document.getElementById('eventId').value;
            
            if (!confirm('Are you sure you want to clear the entire queue?')) {
                return;
            }
            
            const response = await fetch('/api/clear?eventId=' + eventId, {
                method: 'POST'
            });
            
            const state = await response.json();
            updateQueueState(state);
        }

        document.getElementById('eventId').addEventListener('change', () => {
            connectWebSocket();
        });

        connectWebSocket();
    </script>
</body>
</html>`

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}

func main() {
	initRedis()

	http.HandleFunc("/", handleIndex)
	http.HandleFunc("/ws", handleWebSocket)
	http.HandleFunc("/api/state", handleGetQueueState)
	http.HandleFunc("/api/add", handleAddConnections)
	http.HandleFunc("/api/remove", handleRemoveConnections)
	http.HandleFunc("/api/clear", handleClearQueue)

	port := os.Getenv("SERVER_PORT")

	log.Printf("Queue Control Service starting on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
