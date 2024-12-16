package shared

type TransactionQueue struct {
	Transactions []Transaction
}

func (q *TransactionQueue) Enqueue(tx Transaction) {
	q.Transactions = append(q.Transactions, tx)
}

func (q *TransactionQueue) EnqueueFront(tx Transaction) {
	q.Transactions = append([]Transaction{tx}, q.Transactions...)
}

func (q *TransactionQueue) Dequeue() *Transaction {
	if len(q.Transactions) == 0 {
		return nil
	}

	tx := q.Transactions[0]
	q.Transactions = q.Transactions[1:]

	return &tx
}

func (q *TransactionQueue) IsEmpty() bool {
	return len(q.Transactions) == 0
}

func (q *TransactionQueue) Length() int {
	return len(q.Transactions)
}
