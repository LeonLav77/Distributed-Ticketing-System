package main

import "github.com/golang-jwt/jwt/v5"

type TicketAvailabilityResponse struct {
	EventId          string                    `json:"eventId"`
	AvailableTickets []TicketAvailabilityEntry `json:"availableTickets"`
}

type TicketAvailabilityEntry struct {
	Quantity   int    `json:"quantity"`
	TicketType string `json:"ticketType"`
}

type ReserveTicketsRequest struct {
	EventId    string `json:"eventId"`
	Token      string `json:"token"`
	Quantity   int    `json:"quantity"`
	TicketType string `json:"ticketType"`
}

type TicketReservationResponse struct {
	Success     bool   `json:"success"`
	CheckoutURL string `json:"checkoutUrl,omitempty"`
}

type MockLineItem struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Amount      int64  `json:"amount"`
	Currency    string `json:"currency"`
	Quantity    int64  `json:"quantity"`
}

type MockCheckoutRequest struct {
	LineItems  []MockLineItem    `json:"line_items"`
	SuccessURL string            `json:"success_url"`
	CancelURL  string            `json:"cancel_url"`
	Metadata   map[string]string `json:"metadata"`
}

type MockCheckoutResponse struct {
	ID     string `json:"id"`
	URL    string `json:"url"`
	Status string `json:"status"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

type AdmissionsClaims struct {
	EventID string `json:"event_id"`
	UserID  int    `json:"user_id"`
	jwt.RegisteredClaims
}

type UserClaims struct {
	Username string `json:"username"`
	UserID   int    `json:"user_id"`
	jwt.RegisteredClaims
}