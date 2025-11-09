package main

import (
	"fmt"
	"log"
	"net/http"
)

func handlePaymentSuccess(w http.ResponseWriter, r *http.Request) {
	orderReferenceId := r.URL.Query().Get("order_reference_id")

	if orderReferenceId == "" {
		http.Error(w, "Missing order_reference_id parameter", http.StatusBadRequest)
		return
	}

	log.Printf("Payment successful for order: %s", orderReferenceId)

	messageBody := []byte(fmt.Sprintf(`{"order_reference_id":"%s"}`, orderReferenceId))
	if err := sendRabbitMQMessage("order.payment-success", messageBody); err != nil {
		log.Printf("Failed to send RabbitMQ message: %v", err)
	}

	// Redirect to frontend
	frontendURL := getEnv("FRONTEND_URL", "http://127.0.0.1:8080")
	redirectURL := fmt.Sprintf("%s/order-finished", frontendURL)
	http.Redirect(w, r, redirectURL, http.StatusSeeOther)
}

func handlePaymentCancel(w http.ResponseWriter, r *http.Request) {
	orderReferenceId := r.URL.Query().Get("order_reference_id")

	if orderReferenceId == "" {
		http.Error(w, "Missing order_reference_id parameter", http.StatusBadRequest)
		return
	}

	log.Printf("Payment cancelled for order: %s", orderReferenceId)

	messageBody := []byte(fmt.Sprintf(`{"order_reference_id":"%s","status":"cancelled"}`, orderReferenceId))
	if err := sendRabbitMQMessage("order.payment_cancelled", messageBody); err != nil {
		log.Printf("Failed to send RabbitMQ message: %v", err)
	}

	// Redirect to frontend
	frontendURL := getEnv("FRONTEND_URL", "http://127.0.0.1:8080")
	redirectURL := fmt.Sprintf("%s/order-finished", frontendURL)
	http.Redirect(w, r, redirectURL, http.StatusSeeOther)
}
