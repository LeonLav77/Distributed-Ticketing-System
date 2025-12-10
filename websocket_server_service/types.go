package main

import (
	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
)

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

type Client struct {
	conn    *websocket.Conn
	userID  int
	eventID string
}

type NewQueueUpdateMessage struct {
	EstimatedWaitTime  int    `json:"estimatedWaitTime"`
	PositionInQueue    int    `json:"positionInQueue"`
	PeopleAheadInQueue int    `json:"peopleAheadInQueue"`
	TotalPeopleInQueue int    `json:"totalPeopleInQueue"`
	AcceptedTokenId    string `json:"acceptedTokenId"`
}
