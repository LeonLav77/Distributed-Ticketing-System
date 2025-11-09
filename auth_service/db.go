package main

import (
	"database/sql"
	"fmt"
)

func insertUserInDatabase(username string, hashedPassword string, salt string) (int, error) {
	query := `INSERT INTO users (username, password, password_salt) VALUES ($1, $2, $3) RETURNING id`

	var userID int
	err := db.QueryRow(query, username, hashedPassword, salt).Scan(&userID)
	if err != nil {
		return 0, err
	}

	return userID, nil
}

func getUserByUsername(username string) (*User, error) {
	var user User
	query := `SELECT id, username, password, password_salt FROM users WHERE username = $1`
	err := db.QueryRow(query, username).Scan(&user.ID, &user.Username, &user.Password, &user.Salt)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrUserNotFound
		}

		return nil, err
	}

	return &user, nil
}

func userExists(username string) (bool, error) {
	query := `SELECT id FROM users WHERE username = $1`
	err := db.QueryRow(query, username).Scan()

	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}

		return true, fmt.Errorf("something has gone wrong")
	}

	return true, nil
}
