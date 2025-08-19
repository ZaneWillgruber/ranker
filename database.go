package main

import (
	"database/sql"
	"encoding/json"
	"log"

	_ "github.com/mattn/go-sqlite3"
)

var db *sql.DB

func initDB() {
	var err error

	db, err = sql.Open("sqlite3", "./ratings.db")
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}

	statement, err := db.Prepare(`
		CREATE TABLE IF NOT EXISTS ratings (
			message_id TEXT PRIMARY KEY,
			item_name TEXT NOT NULL,
			votes TEXT
		)
	`)
	if err != nil {
		log.Fatalf("Failed to prepare table creation statement: %v", err)
	}
	_, err = statement.Exec()
	if err != nil {
		log.Fatalf("Failed to create ratings table: %v", err)
	}
	log.Println("Database initialized and table created successfully.")
}

func saveNewRating(messageID, itemName string) {
	votesJSON, _ := json.Marshal(make(map[string]int))

	statement, err := db.Prepare("INSERT INTO ratings (message_id, item_name, votes) VALUES (?, ?, ?)")
	if err != nil {
		log.Printf("Failed to prepare insert statement: %v", err)
		return
	}
	defer statement.Close()
	_, err = statement.Exec(messageID, itemName, string(votesJSON))
	if err != nil {
		log.Printf("Failed to execute insert statement: %v", err)
	}
}

func updateVotes(messageID string, votes map[string]int) {
	votesJSON, err := json.Marshal(votes)
	if err != nil {
		log.Printf("Failed to marshal votes to JSON: %v", err)
		return
	}

	statement, err := db.Prepare("UPDATE ratings SET votes = ? WHERE message_id = ?")
	if err != nil {
		log.Printf("Failed to prepare update statement: %v", err)
		return
	}
	defer statement.Close()
	_, err = statement.Exec(string(votesJSON), messageID)
	if err != nil {
		log.Printf("Failed to execute update statement: %v", err)
	}
}

func loadRatings() {
	rows, err := db.Query("SELECT message_id, item_name, votes FROM ratings")
	if err != nil {
		log.Printf("Failed to query ratings from database: %v", err)
		return
	}
	defer rows.Close()

	ratings.Lock()
	defer ratings.Unlock()

	loadedCount := 0
	for rows.Next() {
		var messageID, itemName, votesJSON string
		if err := rows.Scan(&messageID, &itemName, &votesJSON); err != nil {
			log.Printf("Failed to scan row: %v", err)
			continue
		}

		var votes map[string]int
		if err := json.Unmarshal([]byte(votesJSON), &votes); err != nil {
			log.Printf("Failed to unmarshal votes for message %s: %v", messageID, err)
			continue
		}

		ratings.m[messageID] = &RatingInfo{
			ItemName: itemName,
			Votes:    votes,
		}
		loadedCount++
	}
	log.Printf("Loaded %d ratings from the database.", loadedCount)
}
