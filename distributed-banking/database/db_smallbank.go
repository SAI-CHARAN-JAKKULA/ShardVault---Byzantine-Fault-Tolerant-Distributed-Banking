package database

import (
	"database/sql"
	"fmt"
)

// SmallBank tables creation (you have these already):
// Users: (uid, name)
// Savings: (uid, bal)
// Checking: (uid, bal)

// Initialize the SmallBank tables (if not done already):
func InitSmallBankSchema(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			uid INTEGER PRIMARY KEY,
			name TEXT
		);
		CREATE TABLE IF NOT EXISTS savings (
			uid INTEGER PRIMARY KEY,
			bal INTEGER NOT NULL DEFAULT 0
		);
		CREATE TABLE IF NOT EXISTS checking (
			uid INTEGER PRIMARY KEY,
			bal INTEGER NOT NULL DEFAULT 0
		);`)
	if err != nil {
		return fmt.Errorf("failed to initialize SmallBank schema: %v", err)
	}
	return nil
}

// Helper functions to get balances:
func getCheckingBalance(db *sql.DB, uid int) (int, error) {
	var bal int
	err := db.QueryRow("SELECT bal FROM checking WHERE uid=?", uid).Scan(&bal)
	if err == sql.ErrNoRows {
		// If not found, initialize the user with 0
		_, err = db.Exec("INSERT INTO checking (uid, bal) VALUES (?, 0)", uid)
		if err != nil {
			return 0, err
		}
		return 0, nil
	}
	return bal, err
}

func getSavingsBalance(db *sql.DB, uid int) (int, error) {
	var bal int
	err := db.QueryRow("SELECT bal FROM savings WHERE uid=?", uid).Scan(&bal)
	if err == sql.ErrNoRows {
		// If not found, initialize with 0
		_, err = db.Exec("INSERT INTO savings (uid, bal) VALUES (?, 0)", uid)
		if err != nil {
			return 0, err
		}
		return 0, nil
	}
	return bal, err
}

// 1. Amalgamate(fromUID, toUID):
// Move all of 'fromUID'’s savings into 'toUID'’s checking.
func SmallBankAmalgamate(db *sql.DB, fromUID, toUID int) error {
	fromSavings, err := getSavingsBalance(db, fromUID)
	if err != nil {
		return fmt.Errorf("failed to read fromUID savings: %v", err)
	}

	// Deduct from fromUID savings
	_, err = db.Exec("UPDATE savings SET bal = 0 WHERE uid=?", fromUID)
	if err != nil {
		return fmt.Errorf("failed to clear savings: %v", err)
	}

	// Add to toUID checking
	_, err = db.Exec("UPDATE checking SET bal = bal + ? WHERE uid=?;", fromSavings, toUID)
	if err != nil {
		// If toUID has no row yet, insert one
		if err == sql.ErrNoRows {
			_, err = db.Exec("INSERT INTO checking (uid, bal) VALUES (?, ?)", toUID, fromSavings)
			if err != nil {
				return fmt.Errorf("failed to create checking row for toUID: %v", err)
			}
		} else {
			return fmt.Errorf("failed to deposit to checking: %v", err)
		}
	}
	return nil
}

// 2. Balance(uid):
// Return the sum of checking and savings for the user
func SmallBankBalance(db *sql.DB, uid int) (int, error) {
	checkingBal, err := getCheckingBalance(db, uid)
	if err != nil {
		return 0, fmt.Errorf("failed to get checking balance: %v", err)
	}

	savingsBal, err := getSavingsBalance(db, uid)
	if err != nil {
		return 0, fmt.Errorf("failed to get savings balance: %v", err)
	}

	return checkingBal + savingsBal, nil
}

// 3. DepositChecking(uid, amount):
// Add amount to user’s checking balance
func SmallBankDepositChecking(db *sql.DB, uid int, amount int) error {
	// Ensure user exists in checking:
	_, err := getCheckingBalance(db, uid)
	if err != nil {
		return err
	}
	_, err = db.Exec("UPDATE checking SET bal = bal + ? WHERE uid=?", amount, uid)
	return err
}

// 4. SendPayment(fromUID, toUID, amount):
// Send amount from fromUID’s checking to toUID’s checking
func SmallBankSendPayment(db *sql.DB, fromUID, toUID int, amount int) error {
	// Ensure both users exist in checking:
	_, err := getCheckingBalance(db, fromUID)
	if err != nil {
		return err
	}
	_, err = getCheckingBalance(db, toUID)
	if err != nil {
		return err
	}

	_, err = db.Exec("UPDATE checking SET bal = bal - ? WHERE uid=?", amount, fromUID)
	if err != nil {
		return err
	}
	_, err = db.Exec("UPDATE checking SET bal = bal + ? WHERE uid=?", amount, toUID)
	return err
}

// 5. TransactSavings(uid, amount):
// Add amount to user’s savings. amount can be positive or negative.
func SmallBankTransactSavings(db *sql.DB, uid int, amount int) error {
	_, err := getSavingsBalance(db, uid)
	if err != nil {
		return err
	}
	_, err = db.Exec("UPDATE savings SET bal = bal + ? WHERE uid=?", amount, uid)
	return err
}

// 6. WriteCheck(uid, amount):
// Withdraw amount from checking, possibly going negative.
func SmallBankWriteCheck(db *sql.DB, uid int, amount int) error {
	_, err := getCheckingBalance(db, uid)
	if err != nil {
		return err
	}
	_, err = db.Exec("UPDATE checking SET bal = bal - ? WHERE uid=?", amount, uid)
	return err
}
