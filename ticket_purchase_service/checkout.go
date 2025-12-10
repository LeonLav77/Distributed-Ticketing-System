package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
)

func createCheckoutSession(orderReferenceId string, requestData ReserveTicketsRequest) (string, error) {
	callbackBaseURL := os.Getenv("CALLBACK_BASE_URL")
	successURL := fmt.Sprintf("%s/webhooks/payment-success?order_reference_id=%s", callbackBaseURL, orderReferenceId)
	cancelURL := fmt.Sprintf("%s/webhooks/payment-cancel?order_reference_id=%s", callbackBaseURL, orderReferenceId)

	payload := MockCheckoutRequest{
		LineItems: []MockLineItem{
			{
				Name:        fmt.Sprintf("%s Ticket - Event %s", requestData.TicketType, requestData.EventId),
				Description: fmt.Sprintf("%s tier ticket", requestData.TicketType),
				Amount:      int64(5000),
				Currency:    "usd",
				Quantity:    int64(requestData.Quantity),
			},
		},
		SuccessURL: successURL,
		CancelURL:  cancelURL,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	paymentProcessorURL := os.Getenv("PAYMENT_PROCESSOR_URL")
	checkoutEndpoint := fmt.Sprintf("%s/v1/checkout/sessions", paymentProcessorURL)

	resp, err := http.Post(
		checkoutEndpoint,
		"application/json",
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		return "", fmt.Errorf("failed to call payment API: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("payment API returned status %d", resp.StatusCode)
	}

	var result MockCheckoutResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode payment response: %v", err)
	}

	log.Printf("Created checkout session: %s with URL: %s", result.ID, result.URL)
	return result.URL, nil
}
