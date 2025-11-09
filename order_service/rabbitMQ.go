package main

import (
	"log"
	"os"

	"github.com/streadway/amqp"
)

func createRabbitMQConnection() (*amqp.Connection, error) {
	rabbitMQURL := os.Getenv("RABBITMQ_URL")

	log.Printf("Connecting to RabbitMQ at: %s", rabbitMQURL)

	conn, err := amqp.Dial(rabbitMQURL)
	if err != nil {
		return nil, err
	}

	log.Println("✅ Connected to RabbitMQ")
	return conn, nil
}

func createRabbitMQChannel(conn *amqp.Connection) (*amqp.Channel, error) {
	return conn.Channel()
}

func createStandardRabbitMQQueue(channel *amqp.Channel, queueName string) (amqp.Queue, error) {
	return channel.QueueDeclare(
		queueName,
		true,  // durable
		false, // auto-delete
		false, // exclusive
		false, // no-wait
		nil,   // arguments
	)
}

func createStandardRabbitMQMessagesChannel(channel *amqp.Channel, queue *amqp.Queue) (<-chan amqp.Delivery, error) {
	return channel.Consume(
		queue.Name,
		"",    // consumer tag
		false, // auto-ack (manual acknowledgment)
		false, // exclusive
		false, // no-local
		false, // no-wait
		nil,   // arguments
	)
}
