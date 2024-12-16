package database

import (
	"database/sql"
	"fmt"
	"os"

	_ "github.com/mattn/go-sqlite3"
)

type Database struct {
	DB       *sql.DB
	ServerID string
	ShardID  string
}

func InitDatabase(serverID string, shardIDs []int) (*Database, error) {
	filePath := fmt.Sprintf("db_%s.db", serverID)

	// Check if the database file exists and delete it if it does
	if _, err := os.Stat(filePath); err == nil {
		err = os.Remove(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to delete existing database file for server %s: %v", serverID, err)
		}
	}

	db, err := sql.Open("sqlite3", filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database for server %s: %v", serverID, err)
	}

	// Create a single table for client/shard balances and locks
	_, err = db.Exec(`
	CREATE TABLE IF NOT EXISTS clients (
		client_id INTEGER PRIMARY KEY,
		balance INTEGER NOT NULL,
		lock BOOLEAN NOT NULL DEFAULT 0
	);
	CREATE TABLE IF NOT EXISTS transactions (
		transaction_id TEXT PRIMARY KEY,
		source INTEGER NOT NULL,
		destination INTEGER NOT NULL,
		amount INTEGER NOT NULL,
		sequence_number INTEGER NOT NULL,
		contact_server INTEGER NOT NULL,
		status TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE TABLE IF NOT EXISTS WAL (
		transaction_id TEXT NOT NULL,
		source INTEGER NOT NULL,
		destination INTEGER NOT NULL,
		amount INTEGER NOT NULL,
		sequence_number INTEGER NOT NULL,
		PRIMARY KEY (transaction_id)
	);
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to create tables for server %s: %v", serverID, err)
	}

	// Initialize the clients table with an initial balance of 10 and lock set to false
	for _, shardID := range shardIDs {
		_, err = db.Exec(`
		INSERT OR IGNORE INTO clients (client_id, balance, lock)
		VALUES (?, 10, 0)`, shardID)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize client ID %d for server %s: %v", shardID, serverID, err)
		}
	}

	return &Database{DB: db, ServerID: serverID}, nil
}

func ClearTransactions(db *sql.DB) error {
	_, err := db.Exec("DELETE FROM transactions")
	if err != nil {
		return fmt.Errorf("failed to clear transactions table: %v", err)
	}
	return nil
}
