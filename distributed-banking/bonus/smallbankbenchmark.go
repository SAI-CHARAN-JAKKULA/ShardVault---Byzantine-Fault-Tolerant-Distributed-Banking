package bonus

import (
	"database/sql"
	"fmt"
)

// This file provides example functions for implementing SmallBank transactions.
// SmallBank involves three tables: Users, Savings, Checking. Transactions are:
// - Balance
// - DepositChecking
// - TransactSaving
// - Amalgamate
// - WriteCheck
// - SendPayment

// SmallBankBalance returns the sum of checking and savings balances for a user.
func SmallBankBalance(db *sql.DB, uid int) (int, error) {
	var checkingBal, savingsBal int
	err := db.QueryRow("SELECT bal FROM checking WHERE uid=?", uid).Scan(&checkingBal)
	if err != nil && err != sql.ErrNoRows {
		return 0, fmt.Errorf("failed to get checking balance: %v", err)
	}
	err = db.QueryRow("SELECT bal FROM savings WHERE uid=?", uid).Scan(&savingsBal)
	if err != nil && err != sql.ErrNoRows {
		return 0, fmt.Errorf("failed to get savings balance: %v", err)
	}
	return checkingBal + savingsBal, nil
}

// SmallBankDepositChecking increments the checking balance for a user.
func SmallBankDepositChecking(db *sql.DB, uid int, amount int) error {
	_, err := db.Exec("UPDATE checking SET bal = bal + ? WHERE uid=?", amount, uid)
	return err
}

// SmallBankTransactSaving increments the savings balance for a user.
func SmallBankTransactSaving(db *sql.DB, uid int, amount int) error {
	_, err := db.Exec("UPDATE savings SET bal = bal + ? WHERE uid=?", amount, uid)
	return err
}

// SmallBankAmalgamate transfers all of a user's savings into their checking.
func SmallBankAmalgamate(db *sql.DB, fromUID, toUID int) error {
	var fromSavings int
	err := db.QueryRow("SELECT bal FROM savings WHERE uid=?", fromUID).Scan(&fromSavings)
	if err != nil {
		return fmt.Errorf("failed to read fromUID savings: %v", err)
	}

	// Deduct from fromUID savings
	_, err = db.Exec("UPDATE savings SET bal = 0 WHERE uid=?", fromUID)
	if err != nil {
		return fmt.Errorf("failed to clear savings: %v", err)
	}

	// Add to toUID checking
	_, err = db.Exec("UPDATE checking SET bal = bal + ? WHERE uid=?", fromSavings, toUID)
	if err != nil {
		return fmt.Errorf("failed to deposit to checking: %v", err)
	}
	return nil
}

// SmallBankWriteCheck withdraws amount from checking if possible, otherwise results in negative balance.
func SmallBankWriteCheck(db *sql.DB, uid int, amount int) error {
	_, err := db.Exec("UPDATE checking SET bal = bal - ? WHERE uid=?", amount, uid)
	return err
}

// SmallBankSendPayment sends amount from one user's checking to another's checking.
func SmallBankSendPayment(db *sql.DB, fromUID, toUID int, amount int) error {
	_, err := db.Exec("UPDATE checking SET bal = bal - ? WHERE uid=?", amount, fromUID)
	if err != nil {
		return err
	}
	_, err = db.Exec("UPDATE checking SET bal = bal + ? WHERE uid=?", amount, toUID)
	return err
}

// To integrate these SmallBank operations:
// 1. When reading a transaction from input, if it's a SmallBank type (e.g., "SmallBankDepositChecking"),
//    parse the user IDs and amounts.
// 2. In my server code, when we detect tx.Type starts with "SmallBank", call the corresponding SmallBank
//    function above. For example:
//    if tx.Type == "SmallBankDepositChecking" {
//       err = bonus.SmallBankDepositChecking(s.Database.DB, tx.Source, tx.Amount)
//    }

// Make sure we have the user accounts initialized in the database. we can either initialize them at startup
// or when they first appear in a transaction (e.g., INSERT into checking/savings if no row exists).
