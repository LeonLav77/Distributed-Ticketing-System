package main

type Order struct {
	ID               int      `json:"id"`
	EventID          string   `json:"event_id"`
	OrderReferenceID string   `json:"order_reference_id"`
	Status           string   `json:"status"`
	TotalQuantity    int      `json:"total_quantity"`
	Event            Event    `json:"event"`
	Tickets          []Ticket `json:"tickets"`
}

type Ticket struct {
	ID         int     `json:"id"`
	TicketType string  `json:"ticket_type"`
	SeatNumber string  `json:"seat_number"`
	Price      float64 `json:"price"`
}

type Event struct {
	DisplayImage  string `json:"display_image"`
	VenueName     string `json:"venue_name"`
	PerformerName string `json:"performer_name"`
}

type DirectusEvent struct {
	Data struct {
		DisplayImage string `json:"display_image"`
		Venue        []struct {
			VenuesID struct {
				Name string `json:"Name"`
			} `json:"venues_id"`
		} `json:"venue"`
		Performer []struct {
			PerformersID struct {
				Name string `json:"Name"`
			} `json:"performers_id"`
		} `json:"performer"`
	} `json:"data"`
}
