package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"os"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/streadway/amqp"
)

type OrderCreatedMessage struct {
	EventID          string `json:"event_id"`
	TicketType       string `json:"ticket_type"`
	Quantity         int    `json:"quantity"`
	UserID           int    `json:"user_id"`
	OrderReferenceId string `json:"order_reference_id"`
}

type OrderPaymentSuccessMessage struct {
	OrderReferenceId string `json:"order_reference_id"`
}

type TicketData struct {
	SeatNumber string
	Price      float64
}

var (
	db *sql.DB
)

func main() {
	godotenv.Load()

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
	defer db.Close()

	rabbitMQConnection, err := createRabbitMQConnection()
	if err != nil {
		panic(err)
	}
	defer rabbitMQConnection.Close()

	rabbitMQChannel, err := createRabbitMQChannel(rabbitMQConnection)
	if err != nil {
		panic(err)
	}
	defer rabbitMQChannel.Close()

	orderSuccessQueueName := os.Getenv("QUEUE_ORDER_PAYMENT_SUCCESS")
	orderCreatedQueueName := os.Getenv("QUEUE_ORDER_CREATED")

	log.Printf("Setting up queues: %s, %s", orderSuccessQueueName, orderCreatedQueueName)

	orderSuccessQueue, err := createStandardRabbitMQQueue(rabbitMQChannel, orderSuccessQueueName)
	if err != nil {
		panic(err)
	}

	orderSuccessMessages, err := createStandardRabbitMQMessagesChannel(rabbitMQChannel, &orderSuccessQueue)
	if err != nil {
		panic(err)
	}

	orderCreatedQueue, err := createStandardRabbitMQQueue(rabbitMQChannel, orderCreatedQueueName)
	if err != nil {
		panic(err)
	}

	orderCreatedMessages, err := createStandardRabbitMQMessagesChannel(rabbitMQChannel, &orderCreatedQueue)
	if err != nil {
		panic(err)
	}

	forever := make(chan struct{})

	go handleOrderSuccessMessages(orderSuccessMessages)
	go handleOrderCreatedMessages(orderCreatedMessages)

	<-forever
}

func handleOrderCreatedMessages(orderCreatedMessages <-chan amqp.Delivery) {
	for message := range orderCreatedMessages {
		log.Printf("📝 Received order created: %s\n", string(message.Body))

		data := OrderCreatedMessage{}
		err := json.Unmarshal(message.Body, &data)
		if err != nil {
			log.Printf("Failed to unmarshal message: %v\n", err)
			message.Nack(false, false)
			continue
		}

		log.Printf("Received order created message: %+v\n", data)

		tx, err := db.Begin()
		if err != nil {
			log.Printf("Failed to begin transaction: %v\n", err)
			message.Nack(false, false)
			continue
		}

		query := `INSERT INTO orders (user_id, event_id, order_reference_id, total_quantity) 
		          VALUES ($1, $2, $3, $4) RETURNING id`
		var orderID int

		err = tx.QueryRow(query, data.UserID, data.EventID, data.OrderReferenceId, data.Quantity).Scan(&orderID)
		if err != nil {
			log.Printf("Failed to insert order: %v\n", err)
			tx.Rollback()
			message.Nack(false, false)
			continue
		}

		for i := 0; i < data.Quantity; i++ {
			ticket := generateTicketData()

			ticketQuery := `INSERT INTO tickets 
			                (order_id, ticket_type, seat_number, price) 
			                VALUES ($1, $2, $3, $4)`

			_, err = tx.Exec(ticketQuery,
				orderID,
				data.TicketType,
				ticket.SeatNumber,
				ticket.Price,
			)

			if err != nil {
				log.Printf("Failed to insert ticket: %v\n", err)
				tx.Rollback()
				message.Nack(false, false)
				break
			}
		}

		if err != nil {
			continue
		}

		err = tx.Commit()
		if err != nil {
			log.Printf("Failed to commit transaction: %v\n", err)
			message.Nack(false, false)
			continue
		}

		log.Printf("Order created successfully with ID %d for user ID %d with %d tickets\n",
			orderID, data.UserID, data.Quantity)

		message.Ack(false)
	}
}

func handleOrderSuccessMessages(orderSuccessMessages <-chan amqp.Delivery) {
	for message := range orderSuccessMessages {
		log.Printf("📩 Received order payment success: %s\n", string(message.Body))

		data := OrderPaymentSuccessMessage{}
		err := json.Unmarshal(message.Body, &data)
		if err != nil {
			log.Printf("Failed to unmarshal message: %v\n", err)
			message.Nack(false, false)
			continue
		}

		log.Printf("Received order payment success message: %+v\n", data)

		query := `UPDATE orders SET status = 'completed' WHERE order_reference_id = $1`
		_, err = db.Exec(query, data.OrderReferenceId)
		if err != nil {
			log.Printf("Failed to update order status: %v\n", err)
			message.Nack(false, false)
			continue
		}

		log.Printf("Order status updated to completed for order reference ID %s\n", data.OrderReferenceId)

		message.Ack(false)
	}
}


func generateTicketData() TicketData {
	price := 50.0

	return TicketData{
		SeatNumber: fmt.Sprintf("%d", rand.Intn(500)+1),
		Price:      price,
	}
}