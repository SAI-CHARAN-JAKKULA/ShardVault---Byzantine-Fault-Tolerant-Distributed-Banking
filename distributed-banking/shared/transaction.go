package shared

// Transaction represents a transaction between two servers
type Transaction struct {
	TransactionID  string // Unique transaction ID
	Source         int    // T
	Destination    int    //
	Amount         int    // The amount of the transaction
	ContactServer  int
	Status         string // Status of the transaction (e.g., "pending", "committed","local","failed")
	SequenceNumber int    // Sequence number for the transaction
	Signature      []byte // Digital signature of the transaction
}
