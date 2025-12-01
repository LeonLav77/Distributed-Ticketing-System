package main

import (
	"log"
	"net/http"
	"os"
	"strings"
	"sync/atomic"

	"github.com/gorilla/websocket"
)

type LoadBalancer struct {
	backends []string
	current  atomic.Uint64
}

func NewLoadBalancer(backends []string) *LoadBalancer {
	lb := &LoadBalancer{
		backends: backends,
	}
	return lb
}

func (lb *LoadBalancer) nextBackend() string {
	idx := lb.current.Add(1) % uint64(len(lb.backends))
	return lb.backends[idx]
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
}

func (lb *LoadBalancer) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	clientConn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Failed to upgrade: %v", err)
		return
	}

	backendURL := lb.nextBackend() + r.URL.Path
	if r.URL.RawQuery != "" {
		backendURL += "?" + r.URL.RawQuery
	}

	log.Printf("Routing to backend: %s", backendURL)

	headers := http.Header{}
	for key, values := range r.Header {
		if key == "Cookie" || key == "Authorization" || key == "User-Agent" {
			for _, value := range values {
				headers.Add(key, value)
			}
		}
	}

	dialer := websocket.DefaultDialer
	backendConn, _, err := dialer.Dial(backendURL, headers)
	if err != nil {
		log.Printf("Failed to connect to backend %s: %v", backendURL, err)
		clientConn.Close()
		return
	}

	done := make(chan struct{})

	// Proxy backend to client
	go proxyWebSocket(clientConn, backendConn, done)
	// Proxy client to backend
	go proxyWebSocket(backendConn, clientConn, done)

	<-done

	clientConn.Close()
	backendConn.Close()
}

func proxyWebSocket(src *websocket.Conn, dst *websocket.Conn, done chan struct{}) {
	defer func() {
		select {
		case done <- struct{}{}:
		default:
		}
	}()

	for {
		messageType, message, err := src.ReadMessage()
		if err != nil {
			return
		}
		if err := dst.WriteMessage(messageType, message); err != nil {
			return
		}
	}
}

func getBackends() []string {
	backendsStr := os.Getenv("BACKENDS")
	if backendsStr == "" {
		return []string{}
	}
	
	backends := strings.Split(backendsStr, ",")
	for i := range backends {
		backends[i] = strings.TrimSpace(backends[i])
	}
	
	return backends
}

func main() {
	backends := getBackends()
	
	if len(backends) == 0 {
		log.Fatal("No backends configured. Set BACKENDS (comma-separated) environment variable")
	}
	
	log.Printf("Configured %d backends:", len(backends))
	for i, backend := range backends {
		log.Printf("  [%d] %s", i+1, backend)
	}

	lb := NewLoadBalancer(backends)
	http.HandleFunc("/ws", lb.HandleWebSocket)

	port := os.Getenv("LB_PORT")

	log.Printf("WebSocket Load Balancer starting on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}