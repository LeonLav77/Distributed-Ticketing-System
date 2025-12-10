package main

type QueueState struct {
	EventID      string   `json:"eventId"`
	TotalInQueue int      `json:"totalInQueue"`
	UserIDs      []string `json:"userIds"`
}

type AddConnectionsRequest struct {
	EventID string `json:"eventId"`
	Count   int    `json:"count"`
}

type RemoveConnectionsRequest struct {
	EventID string `json:"eventId"`
	Count   int    `json:"count"`
}
