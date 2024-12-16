package database

import (
	"database/sql"
	"distributed-banking/shared"
	"fmt"
)

// Fetches the balance for a specific client ID
func GetClientBalance(db *sql.DB, clientID int) (int, error) {
	var balance int
	row := db.QueryRow(`SELECT balance FROM clients WHERE client_id = ?`, clientID)
	err := row.Scan(&balance)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, fmt.Errorf("client ID %d not found", clientID)
		}
		return 0, err
	}
	return balance, nil
}

// Updates the balance for a specific client ID
func UpdateClientBalance(db *sql.DB, clientID int, amount int) error {
	_, err := db.Exec(`UPDATE clients SET balance = balance + ? WHERE client_id = ?`, amount, clientID)
	if err != nil {
		return fmt.Errorf("failed to update balance for client ID %d: %v", clientID, err)
	}
	return nil
}

// Inserts a new transaction into the transactions table with a timestamp
func AddTransaction(db *sql.DB, txID string, source int, destination int, amount int, sequence_number int, contact_server int, status string) error {
	// fmt.Printf("Adding transaction: ID=%s, Source=%d, Destination=%d, Amount=%d, Status=%s\n", txID, source, destination, amount, status)
	_, err := db.Exec(`
		INSERT INTO transactions (transaction_id, source, destination, amount, sequence_number, contact_server, status, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)`,
		txID, source, destination, amount, sequence_number, contact_server, status,
	)
	if err != nil {
		return fmt.Errorf("failed to add transaction %s: %v", txID, err)
	}
	return nil
}

// Sets the lock for a specific client ID
func SetLock(db *sql.DB, clientID int) error {
	_, err := db.Exec(`UPDATE clients SET lock = 1 WHERE client_id = ?`, clientID)
	if err != nil {
		return fmt.Errorf("failed to set lock for client ID %d: %v", clientID, err)
	}
	return nil
}

// Unsets the lock for a specific client ID
func UnsetLock(db *sql.DB, clientID int) error {
	_, err := db.Exec(`UPDATE clients SET lock = 0 WHERE client_id = ?`, clientID)
	if err != nil {
		return fmt.Errorf("failed to unset lock for client ID %d: %v", clientID, err)
	}
	return nil
}

// Checks if a specific client ID is locked
func IsLocked(db *sql.DB, clientID int) (bool, error) {
	var isLocked bool
	row := db.QueryRow(`SELECT lock FROM clients WHERE client_id = ?`, clientID)
	err := row.Scan(&isLocked)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, fmt.Errorf("client ID %d not found", clientID)
		}
		return false, err
	}
	return isLocked, nil
}

// GetAllTransactions retrieves all transactions from the database, ordered by creation time
func GetAllTransactions(db *sql.DB) ([]map[string]interface{}, error) {
	rows, err := db.Query(`
		SELECT transaction_id, source, destination, amount, sequence_number, contact_server, status, created_at
		FROM transactions
		ORDER BY created_at ASC`)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch transactions: %v", err)
	}
	defer rows.Close()

	transactions := make([]map[string]interface{}, 0)
	for rows.Next() {
		var txID, status, createdAt string
		var source, destination, amount, sequence_number, contact_server int
		err := rows.Scan(&txID, &source, &destination, &amount, &sequence_number, &contact_server, &status, &createdAt)
		if err != nil {
			return nil, err
		}
		transactions = append(transactions, map[string]interface{}{
			"transaction_id":  txID,
			"source":          source,
			"destination":     destination,
			"amount":          amount,
			"sequence_number": sequence_number,
			"contact_server":  contact_server,
			"status":          status,
			"created_at":      createdAt,
		})
	}
	return transactions, nil
}

// GetTransaction retrieves a transaction from the database by its ID
func GetTransaction(db *sql.DB, transactionID string) (shared.Transaction, error) {
	var transaction shared.Transaction

	query := `SELECT transaction_id, source, destination, amount, sequence_number, contact_server, status 
			  FROM transactions 
			  WHERE transaction_id = ?`

	row := db.QueryRow(query, transactionID)
	err := row.Scan(
		&transaction.TransactionID,
		&transaction.Source,
		&transaction.Destination,
		&transaction.Amount,
		&transaction.SequenceNumber,
		&transaction.ContactServer,
		&transaction.Status,
	)

	if err == sql.ErrNoRows {
		return transaction, fmt.Errorf("transaction not found: %s", transactionID)
	}
	if err != nil {
		return transaction, fmt.Errorf("error fetching transaction: %v", err)
	}

	return transaction, nil
}

// Adds a new entry to the Write-Ahead Log
func AddToWAL(db *sql.DB, transactionID string, source int, destination int, amount int, sequenceNumber int) error {
	_, err := db.Exec(`
		INSERT INTO WAL (transaction_id, source, destination, amount, sequence_number)
		VALUES (?, ?, ?, ?, ?)`,
		transactionID, source, destination, amount, sequenceNumber,
	)
	if err != nil {
		return fmt.Errorf("failed to add to WAL for transaction ID %s: %v", transactionID, err)
	}
	return nil
}

// Deletes an entry from the Write-Ahead Log by transaction ID
func DeleteFromWAL(db *sql.DB, transactionID string) error {
	_, err := db.Exec(`DELETE FROM WAL WHERE transaction_id = ?`, transactionID)
	if err != nil {
		return fmt.Errorf("failed to delete from WAL for transaction ID %s: %v", transactionID, err)
	}
	return nil
}
