package bonus

import (
	"distributed-banking/database"
	"fmt"
	"time"
)

// MultiDestTransaction represents a transaction that can have multiple destinations.
type MultiDestTransaction struct {
	TransactionID string
	Source        int
	Destinations  []int
	Amounts       []int
	Status        string
}

// ValidateMultiDestTransaction checks if a multi-destination transaction has valid funds.
func ValidateMultiDestTransaction(db *database.Database, tx MultiDestTransaction) error {
	// Calculate total amount
	totalAmt := 0
	for _, amt := range tx.Amounts {
		totalAmt += amt
	}

	// Check sender's balance
	senderBalance, err := database.GetClientBalance(db.DB, tx.Source)
	if err != nil {
		return fmt.Errorf("error fetching balance for source %d: %v", tx.Source, err)
	}
	if senderBalance < totalAmt {
		return fmt.Errorf("insufficient balance for source %d, needs %d", tx.Source, totalAmt)
	}
	return nil
}

// LockClientsForMultiDest locks all participants (source + destinations)
func LockClientsForMultiDest(db *database.Database, tx MultiDestTransaction) error {
	// Lock source
	if err := waitAndLock(db, tx.Source); err != nil {
		return err
	}

	// Lock each destination
	for _, dest := range tx.Destinations {
		if err := waitAndLock(db, dest); err != nil {
			// If lock fails, unlock previously locked clients
			_ = database.UnsetLock(db.DB, tx.Source)
			for _, d := range tx.Destinations {
				_ = database.UnsetLock(db.DB, d)
			}
			return err
		}
	}
	return nil
}

// CommitMultiDestTransaction applies the transaction to the database.
func CommitMultiDestTransaction(db *database.Database, tx MultiDestTransaction, sequenceNumber int, contactServer int) error {
	// Deduct from source
	totalAmt := 0
	for _, amt := range tx.Amounts {
		totalAmt += amt
	}
	if err := database.UpdateClientBalance(db.DB, tx.Source, -totalAmt); err != nil {
		return fmt.Errorf("failed to update source balance: %v", err)
	}

	// Add to each destination
	for i, dest := range tx.Destinations {
		if err := database.UpdateClientBalance(db.DB, dest, tx.Amounts[i]); err != nil {
			return fmt.Errorf("failed to update destination %d balance: %v", dest, err)
		}
	}

	// Record in transactions table - record one entry or multiple entries as desired.
	// or a different schema.
	err := database.AddTransaction(db.DB, tx.TransactionID, tx.Source, tx.Destinations[0], totalAmt, sequenceNumber, contactServer, "committed")
	if err != nil {
		return fmt.Errorf("failed to add transaction record: %v", err)
	}

	// Unlock all clients after commit
	_ = database.UnsetLock(db.DB, tx.Source)
	for _, d := range tx.Destinations {
		_ = database.UnsetLock(db.DB, d)
	}

	return nil
}

// waitAndLock waits until a client is unlocked, then locks them.
func waitAndLock(db *database.Database, clientID int) error {
	clientLocked, err := database.IsLocked(db.DB, clientID)
	for err != nil || clientLocked {
		time.Sleep(100 * time.Millisecond)
		clientLocked, err = database.IsLocked(db.DB, clientID)
	}
	return database.SetLock(db.DB, clientID)
}

// we would integrate these functions by:
// 1. Creating a MultiDestTransaction from your input.
// 2. Calling ValidateMultiDestTransaction and LockClientsForMultiDest.
// 3. Running PBFT and 2PC as before, but with all involved shards.
// 4. Calling CommitMultiDestTransaction once the transaction is decided to commit.

// For cross-shard logic, we can send the MultiDestTransaction info to other shards similar to my existing
// cross-shard transaction handling. Just ensure that we handle multiple destinations rather than just one.
