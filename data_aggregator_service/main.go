package main

import (
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

var (
	db *sql.DB
	directusURL   string
	directusToken string
)

func main() {
	godotenv.Load()

	initDB()
	directusURL = os.Getenv("DIRECTUS_API_URL")
	directusToken = os.Getenv("DIRECTUS_TOKEN")


	http.Handle("/profile", withCORS(http.HandlerFunc(getUserProfile)))

	port := os.Getenv("SERVER_PORT")

	log.Printf("Data Aggregator Service starting on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func initDB() {
	connStr := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		os.Getenv("DB_HOST"),
		os.Getenv("DB_PORT"),
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_NAME"),
		os.Getenv("DB_SSLMODE"),
	)

	log.Printf("Connecting to PostgreSQL at %s:%s...",
		os.Getenv("DB_HOST"),
		os.Getenv("DB_PORT"),
	)

	var err error
	db, err = sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}
}

func getUserProfile(w http.ResponseWriter, r *http.Request) {
	authHeader := r.Header.Get("Authorization")

	userID := extractUserIDFromJWT(authHeader)

	log.Printf("Fetching profile for user ID: %d", userID)
	orders, err := getOrdersForUser(userID)
	if err != nil {
		log.Printf("Failed to fetch orders: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	for i := range orders {
		event, err := getEventFromDirectus(orders[i].EventID)
		if err != nil {
			log.Printf("Failed to fetch event data for event %s: %v", orders[i].EventID, err)
			continue
		}
		orders[i].Event = event

		tickets, err := getTicketsForOrder(orders[i].ID)
		if err != nil {
			log.Printf("Failed to fetch tickets for order %d: %v", orders[i].ID, err)
			continue
		}
		orders[i].Tickets = tickets
	}

	response := map[string]any{
		"user_id": userID,
		"orders":  orders,
	}

	respondWithJSON(w, http.StatusOK, response)
}

func getOrdersForUser(userID int) ([]Order, error) {
	query := `SELECT id, event_id, order_reference_id, status, total_quantity 
	          FROM orders WHERE user_id = $1 ORDER BY created_at DESC`

	rows, err := db.Query(query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orders []Order
	for rows.Next() {
		var order Order
		err := rows.Scan(
			&order.ID,
			&order.EventID,
			&order.OrderReferenceID,
			&order.Status,
			&order.TotalQuantity,
		)
		if err != nil {
			log.Printf("Failed to scan order: %v", err)
			continue
		}

		orders = append(orders, order)
	}

	return orders, nil
}

func getTicketsForOrder(orderID int) ([]Ticket, error) {
	query := `SELECT id, ticket_type, seat_number, price 
	          FROM tickets WHERE order_id = $1`

	rows, err := db.Query(query, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tickets []Ticket
	for rows.Next() {
		var ticket Ticket
		err := rows.Scan(
			&ticket.ID,
			&ticket.TicketType,
			&ticket.SeatNumber,
			&ticket.Price,
		)
		if err != nil {
			log.Printf("Failed to scan ticket: %v", err)
			continue
		}
		tickets = append(tickets, ticket)
	}

	return tickets, nil
}

func getEventFromDirectus(eventID string) (Event, error) {
	url := fmt.Sprintf("%s/items/concerts/%s?fields=display_image,venue.venues_id.Name,performer.performers_id.Name", directusURL, eventID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return Event{}, err
	}

	req.Header.Set("Authorization", "Bearer "+directusToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return Event{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return Event{}, fmt.Errorf("directus returned status %d", resp.StatusCode)
	}

	var directusEvent DirectusEvent
	err = json.NewDecoder(resp.Body).Decode(&directusEvent)
	if err != nil {
		return Event{}, err
	}

	venueName := directusEvent.Data.Venue[0].VenuesID.Name
	performerName := directusEvent.Data.Performer[0].PerformersID.Name
	imageBase64, _ := fetchImageAsBase64(directusURL, directusToken, directusEvent.Data.DisplayImage)

	return Event{
		DisplayImage:  imageBase64,
		VenueName:     venueName,
		PerformerName: performerName,
	}, nil
}

func fetchImageAsBase64(directusURL, token, imageID string) (string, error) {
	url := fmt.Sprintf("%s/assets/%s", directusURL, imageID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}

	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to fetch image: status %d", resp.StatusCode)
	}

	imageBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "image/jpeg"
	}

	base64String := base64.StdEncoding.EncodeToString(imageBytes)

	return fmt.Sprintf("data:%s;base64,%s", contentType, base64String), nil
}

func respondWithJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}