package main

import (
	"log"
	"net/http"
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

	backendConn, _, err := websocket.DefaultDialer.Dial(backendURL, nil)
	if err != nil {
		log.Printf("Failed to connect to backend: %v", err)
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

func main() {
	backends := []string{
		"ws://192.168.1.74:12000",
		"ws://192.168.1.74:12001",
		"ws://192.168.1.74:12002",
	}

	lb := NewLoadBalancer(backends)
	http.HandleFunc("/ws", lb.HandleWebSocket)

	log.Println("WebSocket Load Balancer starting on :4321")
	log.Fatal(http.ListenAndServe(":4321", nil))
}
