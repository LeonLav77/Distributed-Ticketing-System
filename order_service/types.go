package main

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
