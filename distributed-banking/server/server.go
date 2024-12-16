package server

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"distributed-banking/database"
	"distributed-banking/shared"
	"fmt"
	"net"
	"net/rpc"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/hashicorp/vault/shamir"
)

type TransactionRequest struct {
	Transaction      shared.Transaction
	ActiveServers    []string
	ByzantineServers []string
	PublicKey        *rsa.PublicKey
}

type TwoPCPrepareRequest struct {
	CommittedTransactions []shared.Transaction
	ClientTransaction     shared.Transaction
	ActiveServers         []string
	ByzantineServers      []string
	PublicKey             *rsa.PublicKey
}

type Server struct {
	ID                    string // Unique identifier for the server
	Name                  string
	Address               string
	ClusterID             string     // The cluster to which the server belongs
	mu                    sync.Mutex // Mutex for thread safety
	MaxSequenceNumber     int        // Maximum sequence number, initialized to zero
	MaxExecutedSno        int        // Maximum executed sequence number
	PrivateKey            *rsa.PrivateKey
	PublicKey             *rsa.PublicKey
	Database              *database.Database // Database associated with the server
	Transactions          []shared.Transaction
	CommittedTransactions []shared.Transaction
	totalTransactionTime  time.Duration // Total time taken for transactions
}

func (s *Server) HandleTransaction(request TransactionRequest, reply *string) error {
	startTime := time.Now() // Start measuring transaction processing time
	s.mu.Lock()
	tx := request.Transaction
	publicKey := request.PublicKey
	s.totalTransactionTime += time.Since(startTime)

	// pbft code
	if tx.Status == "client-request" {
		if !s.verifyPublicKey(tx, publicKey, reply) {
			s.mu.Unlock()
			fmt.Printf("returning nill at client-request verify")
			return nil
		}

		// Check sender's balance first
		senderBalance, err := database.GetClientBalance(s.Database.DB, tx.Source)
		if err != nil || senderBalance < tx.Amount {
			return fmt.Errorf("[DEBUG] insufficient balance for Sender %d", tx.Source)
		}

		// Wait for locks if sender is locked
		senderLocked, err := database.IsLocked(s.Database.DB, tx.Source)
		for err != nil || senderLocked {
			time.Sleep(100 * time.Millisecond) // Wait for 100 milliseconds
			senderLocked, err = database.IsLocked(s.Database.DB, tx.Source)
		}

		// Wait for locks if receiver is locked
		receiverLocked, err := database.IsLocked(s.Database.DB, tx.Destination)
		for err != nil || receiverLocked {
			time.Sleep(100 * time.Millisecond) // Wait for 100 milliseconds
			receiverLocked, err = database.IsLocked(s.Database.DB, tx.Destination)
		}

		// Lock the sender and receiver
		if err := database.SetLock(s.Database.DB, tx.Source); err != nil {
			return fmt.Errorf("failed to lock Sender %d: %v", tx.Source, err)
		}
		if err := database.SetLock(s.Database.DB, tx.Destination); err != nil {
			return fmt.Errorf("failed to lock Receiver %d: %v", tx.Destination, err)
		}
		tx.Status = "pre-prepare"
		s.MaxSequenceNumber++
		tx.SequenceNumber = s.MaxSequenceNumber
		// fmt.Printf("received client-request at %s tx: %d->%d:%d sqno: %d\n", s.Name, tx.Source, tx.Destination, tx.Amount, tx.SequenceNumber)

		s.Transactions = append(s.Transactions, tx)
		signedTx, err := s.signTransaction(tx)
		if err != nil {
			*reply = "signing failed"
			s.mu.Unlock()
			fmt.Printf("returning nill at client-request sign")
			return nil
		}
		tx.Signature = signedTx
		s.mu.Unlock()
		go s.PrePrepare(tx, request.ActiveServers, request.ByzantineServers)
	} else if tx.Status == "pre-prepare" {
		if !s.verifyPublicKey(tx, publicKey, reply) {
			s.mu.Unlock()
			fmt.Printf("returning nill at pre-prepare verify")
			return nil
		}
		s.Transactions = append(s.Transactions, tx)
		// fmt.Printf("received pre-prepare at %s tx: %d->%d:%d sqno: %d\n", s.Name, tx.Source, tx.Destination, tx.Amount, tx.SequenceNumber)
		if !shared.IsByzantine(request.ByzantineServers, s.Name) {
			tx.Status = "prepare"
			signedTx, err := s.signTransaction(tx)
			if err != nil {
				*reply = "signing failed"
				s.mu.Unlock()
				fmt.Printf("returning nill at pre-prepare sign")
				return nil
			}
			tx.Signature = signedTx
			s.mu.Unlock()
			go s.sendFromBackupsToPrimary(tx, request.ActiveServers, request.ByzantineServers)
		} else {
			s.mu.Unlock()
		}
	} else if tx.Status == "prepare" {
		if !s.verifyPublicKey(tx, publicKey, reply) {
			s.mu.Unlock()
			fmt.Printf("returning nill")
			return nil
		}
		s.Transactions = append(s.Transactions, tx)
		fmt.Printf("received prepare at %s tx: %d->%d:%d sqno: %d\n", s.Name, tx.Source, tx.Destination, tx.Amount, tx.SequenceNumber)
		if !shared.IsByzantine(request.ByzantineServers, s.Name) {
			prepareTransactions := []shared.Transaction{}
			for _, transaction := range s.Transactions {
				if transaction.TransactionID == tx.TransactionID && transaction.Status == "prepare" {
					prepareTransactions = append(prepareTransactions, transaction)
				}
			}
			// Check if enough prepare messages have been received
			if len(prepareTransactions) >= 2 {
				// Implement Shamir's Secret Sharing to create a threshold signature from prepare transactions
				signatures := [][]byte{}
				for _, transaction := range prepareTransactions {
					signatures = append(signatures, transaction.Signature)
				}

				thresholdSignature, err := shamir.Combine(signatures) // TSS Combined Signature
				newTx := tx
				newTx.Status = "prepared"
				newTx.Signature = thresholdSignature
				s.Transactions = append(s.Transactions, newTx)
				go s.sendFromPrimaryWithThresholdKey(newTx, request.ActiveServers, request.ByzantineServers, s.PublicKey)
				if err != nil {
				}
			}
		}
		s.mu.Unlock()
	} else if tx.Status == "prepared" {
		if !shared.IsByzantine(request.ByzantineServers, s.Name) {
			if !s.verifyPublicKey(tx, publicKey, reply) {
				s.mu.Unlock()
				fmt.Printf("returning nill")
				return nil
			}
			s.Transactions = append(s.Transactions, tx)
			fmt.Printf("received prepared at %s tx: %d->%d:%d sqno: %d\n", s.Name, tx.Source, tx.Destination, tx.Amount, tx.SequenceNumber)
			tx.Status = "commit"
			signedTx, err := s.signTransaction(tx)
			if err != nil {
				*reply = "signing failed"
				s.mu.Unlock()
				return nil
			}
			tx.Signature = signedTx
			s.mu.Unlock()
			go s.sendFromBackupsToPrimary(tx, request.ActiveServers, request.ByzantineServers)
		} else {
			s.mu.Unlock()
		}
	} else if tx.Status == "commit" { // at leader
		if !shared.IsByzantine(request.ByzantineServers, s.Name) {
			if !s.verifyPublicKey(tx, publicKey, reply) {
				s.mu.Unlock()
				fmt.Printf("returning nill")
				return nil
			}
			s.Transactions = append(s.Transactions, tx)
			// Create a list of commit transactions
			commitTransactions := []shared.Transaction{}
			for _, transaction := range s.Transactions {
				if transaction.TransactionID == tx.TransactionID && transaction.Status == "commit" {
					commitTransactions = append(commitTransactions, transaction)
				}
			}

			// Check if enough commit messages have been received
			if len(commitTransactions) >= 2 {
				// Implement Shamir's Secret Sharing to create a threshold signature from commit transactions
				// fmt.Printf("received %d commits at %s tx: %s->%s:%d\n", len(commitTransactions), s.Name, tx.Source, tx.Destination, tx.Amount)
				signatures := [][]byte{}
				for _, transaction := range commitTransactions {
					signatures = append(signatures, transaction.Signature)
				}
				thresholdSignature, err := shamir.Combine(signatures)
				newTx := tx
				newTx.Status = "committed"
				newTx.Signature = thresholdSignature
				// Add committed transaction
				s.Transactions = append(s.Transactions, newTx)
				s.CommittedTransactions = append(s.CommittedTransactions, newTx)
				go s.sendFromPrimaryWithThresholdKey(newTx, request.ActiveServers, request.ByzantineServers, s.PublicKey)
				// fmt.Printf("Committing transaction locally: %+v\n", newTx)
				// Execute all committed transactions in order
				count := s.MaxExecutedSno
				for {
					found := false
					for i, transaction := range s.CommittedTransactions {
						if transaction.SequenceNumber == count+1 {
							s.mu.Lock()
							s.CommittedTransactions[i].Status = "executed"
							// fmt.Printf("Executing transaction %d from %d to %d for amount %d\n", transaction.SequenceNumber, transaction.Source, transaction.Destination, transaction.Amount)
							if shared.IsTransactionCrossShard(transaction.Source, transaction.Destination) {
								// Add to WAL if it is crossshard
								if err := s.Database.AddToWAL(s.Database.DB, transaction.TransactionID, transaction.Source, transaction.Destination, transaction.Amount, transaction.SequenceNumber); err != nil {
									return fmt.Errorf("[ERROR] Failed to add to WAL for transaction %s: %v", transaction.TransactionID, err)
								}
							}
							// persist in db
							if err := s.CommitTransactionInDB(transaction); err != nil {
								return fmt.Errorf("[ERROR] Failed to commit transaction in DB: %v", err)
							}
							// Check if it is an intrashard transaction before unlocking
							if !shared.IsTransactionCrossShard(transaction.Source, transaction.Destination) {
								// if it is intrashard transaction unset locks.
								if err := database.UnsetLock(s.Database.DB, transaction.Source); err != nil {
									return fmt.Errorf("failed to unlock Sender %d: %v", transaction.Source, err)
								}
								if err := database.UnsetLock(s.Database.DB, transaction.Destination); err != nil {
									return fmt.Errorf("failed to unlock Receiver %d: %v", transaction.Destination, err)
								}
							}
							s.Transactions = append(s.Transactions, s.CommittedTransactions[i])
							count++
							found = true
							s.mu.Unlock()
							break
						}
					}
					if !found {
						break
					}
				}
				s.MaxExecutedSno = count
				if err != nil {
				}
			}
		}
		s.mu.Unlock()
	} else if tx.Status == "committed" {
		if !shared.IsByzantine(request.ByzantineServers, s.Name) {
			if !s.verifyPublicKey(tx, publicKey, reply) {
				s.mu.Unlock()
				fmt.Printf("returning nill")
				return nil
			}
			s.Transactions = append(s.Transactions, tx)
			// fmt.Printf("received committed at %s tx: %s->%s:%d sqno: %d\n", s.Name, tx.Source, tx.Destination, tx.Amount, tx.SequenceNumber)
			s.CommittedTransactions = append(s.CommittedTransactions, tx)

			// Execute all committed transactions in order
			count := s.MaxExecutedSno
			for {
				found := false
				for i, transaction := range s.CommittedTransactions {
					if transaction.SequenceNumber == count+1 {
						s.mu.Lock()
						s.CommittedTransactions[i].Status = "executed"
						// fmt.Printf("Executing transaction %d from %d to %d for amount %d\n", transaction.SequenceNumber, transaction.Source, transaction.Destination, transaction.Amount)
						if shared.IsTransactionCrossShard(transaction.Source, transaction.Destination) {
							// Add to WAL if it is crossshard
							if err := s.Database.AddToWAL(s.Database.DB, transaction.TransactionID, transaction.Source, transaction.Destination, transaction.Amount, transaction.SequenceNumber); err != nil {
								return fmt.Errorf("[ERROR] Failed to add to WAL for transaction %s: %v", transaction.TransactionID, err)
							}
						}
						// persist in db
						if err := s.CommitTransactionInDB(transaction); err != nil {
							return fmt.Errorf("[ERROR] Failed to commit transaction in DB: %v", err)
						}
						// Check if it is an intrashard transaction before unlocking
						if !shared.IsTransactionCrossShard(transaction.Source, transaction.Destination) {
							// if it is intrashard transaction unset locks.
							if err := database.UnsetLock(s.Database.DB, transaction.Source); err != nil {
								return fmt.Errorf("failed to unlock Sender %d: %v", transaction.Source, err)
							}
							if err := database.UnsetLock(s.Database.DB, transaction.Destination); err != nil {
								return fmt.Errorf("failed to unlock Receiver %d: %v", transaction.Destination, err)
							}
						}
						s.Transactions = append(s.Transactions, s.CommittedTransactions[i])
						count++
						found = true
						s.mu.Unlock()
						break
					}
				}
				if !found {
					break
				}
			}
			s.MaxExecutedSno = count
		}
		s.mu.Unlock()
	} else {
		s.mu.Unlock()
	}
	*reply = "Transaction successfully handled by the leader."
	return nil
}

func StartServerRPC(serverID string, port string, clusterID string, shardIDs []int) *Server {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		fmt.Printf("Error generating private key for server %s: %v\n", serverID, err)
		return nil
	}
	publicKey := &privateKey.PublicKey
	db, err := database.InitDatabase(serverID, shardIDs)
	if err != nil {
		return nil
	}
	s := &Server{
		ID:                    serverID,
		Name:                  serverID,
		Transactions:          []shared.Transaction{},
		CommittedTransactions: []shared.Transaction{},
		Address:               port,
		MaxSequenceNumber:     0,
		MaxExecutedSno:        0,
		PrivateKey:            privateKey,
		PublicKey:             publicKey,
		ClusterID:             clusterID,
		Database:              db, // Assign the database instance
	}
	err = rpc.RegisterName(fmt.Sprintf("Server.%s", serverID), s)
	if err != nil {
		return nil
	}
	listener, err := net.Listen("tcp", port)
	if err != nil {
		return nil
	}
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				continue
			}
			go rpc.ServeConn(conn)
		}
	}()
	return s
}

func (s *Server) CommittedTransactionsInDB(_ struct{}, reply *[]shared.Transaction) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	transactions, err := database.GetAllTransactions(s.Database.DB)
	if err != nil {
		fmt.Printf("Error getting all transactions from database: %v\n", err)
		return err
	}
	var committedTransactions []shared.Transaction
	for _, tx := range transactions {
		committedTransactions = append(committedTransactions, shared.Transaction{
			TransactionID:  tx["transaction_id"].(string),
			Source:         tx["source"].(int),
			Destination:    tx["destination"].(int),
			Amount:         tx["amount"].(int),
			SequenceNumber: tx["sequence_number"].(int),
			ContactServer:  tx["contact_server"].(int),
			Status:         tx["status"].(string),
		})
	}

	*reply = committedTransactions
	return nil
}

func (s *Server) CheckBalanceAndLockClients(transaction shared.Transaction) error {
	senderLocked, err := database.IsLocked(s.Database.DB, transaction.Source)
	if err != nil || senderLocked {
		return fmt.Errorf("[DEBUG] sender %d is locked or error occurred", transaction.Source)
	}
	receiverLocked, err := database.IsLocked(s.Database.DB, transaction.Destination)
	if err != nil || receiverLocked {
		return fmt.Errorf("[DEBUG] receiver %d is locked or error occurred", transaction.Destination)
	}
	senderBalance, err := database.GetClientBalance(s.Database.DB, transaction.Source)
	if err != nil || senderBalance < transaction.Amount {
		return fmt.Errorf("[DEBUG] insufficient balance for Sender %d", transaction.Source)
	}
	if err := database.SetLock(s.Database.DB, transaction.Source); err != nil {
		return fmt.Errorf("failed to lock Sender %d: %v", transaction.Source, err)
	}
	if err := database.SetLock(s.Database.DB, transaction.Destination); err != nil {
		return fmt.Errorf("failed to lock Receiver %d: %v", transaction.Destination, err)
	}
	return nil
}

func (s *Server) CommitTransactionInDB(transaction shared.Transaction) error {
	if err := database.UpdateClientBalance(s.Database.DB, transaction.Source, -transaction.Amount); err != nil {
		return fmt.Errorf("failed to update balance for Sender %d: %v", transaction.Source, err)
	}
	if err := database.UpdateClientBalance(s.Database.DB, transaction.Destination, transaction.Amount); err != nil {
		return fmt.Errorf("failed to update balance for Receiver %d: %v", transaction.Destination, err)
	}
	if err := database.AddTransaction(s.Database.DB, transaction.TransactionID,
		transaction.Source, transaction.Destination,
		transaction.Amount, transaction.SequenceNumber, transaction.ContactServer, transaction.Status); err != nil {
		return fmt.Errorf("failed to add committed transaction: %v", err)
	}

	return nil
}
func (s *Server) HandleClientCrossShardTransaction(request struct {
	Transaction                         shared.Transaction
	ActiveServersForSourceShard         []string
	ByzantineServersForSourceShard      []string
	ActiveServersForDestinationShard    []string
	ByzantineServersForDestinationShard []string
	PublicKey                           *rsa.PublicKey
}, reply *string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx := request.Transaction
	publicKey := request.PublicKey
	if !s.verifyPublicKey(tx, publicKey, reply) {
		s.mu.Unlock()
		return fmt.Errorf("returning error at coordinator verify")
	}
	// Check sender's balance
	senderBalance, err := database.GetClientBalance(s.Database.DB, tx.Source)
	if err != nil || senderBalance < tx.Amount {
		// Skip the transaction if insufficient balance
		return fmt.Errorf("[DEBUG] insufficient balance for Sender %d", tx.Source)
	}

	// Check if sender is locked
	senderLocked, err := database.IsLocked(s.Database.DB, tx.Source)
	for err != nil || senderLocked {
		// Wait for the lock to be released
		time.Sleep(100 * time.Millisecond)
		senderLocked, err = database.IsLocked(s.Database.DB, tx.Source)
	}

	// Lock the sender
	if err := database.SetLock(s.Database.DB, tx.Source); err != nil {
		return fmt.Errorf("failed to lock Sender %d: %v", tx.Source, err)
	}
	// intiate pbft on coordinator cluster
	go s.PrePrepare(tx, request.ActiveServersForSourceShard, request.ByzantineServersForSourceShard)
	// wait for pbft at coordinator cluster
	time.Sleep(5 * time.Millisecond)
	transactions, err := database.GetAllTransactions(s.Database.DB)
	if err != nil {
		fmt.Printf("Error getting all transactions from database: %v\n", err)
		return err
	}
	var committedTransactions []shared.Transaction
	for _, tx := range transactions {
		committedTransactions = append(committedTransactions, shared.Transaction{
			TransactionID:  tx["transaction_id"].(string),
			Source:         tx["source"].(int),
			Destination:    tx["destination"].(int),
			Amount:         tx["amount"].(int),
			SequenceNumber: tx["sequence_number"].(int),
			ContactServer:  tx["contact_server"].(int),
			Status:         tx["status"].(string),
		})
	}
	// send 2PC prepare for the destination shard cluster servers
	replyChan := make(chan string)
	go func() {
		s.send2PCPrepare(tx, committedTransactions, request.ActiveServersForDestinationShard, request.ByzantineServersForDestinationShard)
		replyChan <- "success"
	}()
	select {
	case replyStatus := <-replyChan:
	case <-time.After(50 * time.Millisecond): //co-od timer
		replyStatus := "timeout"
	}
	// release locks on source shard after sending the 2PC prepared
	if err := database.UnsetLock(s.Database.DB, tx.Source); err != nil {
		return fmt.Errorf("failed to unlock Sender %d: %v", tx.Source, err)
	}

	if replyStatus == "success" {
		// run consensus on commit decision in co-od cluster
		request.Transaction.Status = "2PCcommit"
		// Check if sender is locked
		senderLocked, err := database.IsLocked(s.Database.DB, tx.Source)
		for err != nil || senderLocked {
			// Wait for the lock to be released
			time.Sleep(100 * time.Millisecond)
			senderLocked, err = database.IsLocked(s.Database.DB, tx.Source)
		}

		// Lock the sender
		if err := database.SetLock(s.Database.DB, tx.Source); err != nil {
			return fmt.Errorf("failed to lock Sender %d: %v", tx.Source, err)
		}
		s.PrePrepare(request.Transaction, request.ActiveServersForSourceShard, request.ByzantineServersForSourceShard)
		// send commit decision to participant cluster
		if err := database.UnsetLock(s.Database.DB, tx.Source); err != nil {
			return fmt.Errorf("failed to unlock Sender %d: %v", tx.Source, err)
		}
		go s.send2PCCommit(tx, request.ActiveServersForDestinationShard, request.ByzantineServersForDestinationShard) // Call send2PCCommit on success
	} else {
		// run consensus on abort decision in co-od cluster
		request.Transaction.Status = "2PCabort"
		// Check if sender is locked
		senderLocked, err := database.IsLocked(s.Database.DB, tx.Source)
		for err != nil || senderLocked {
			// Wait for the lock to be released
			time.Sleep(100 * time.Millisecond)
			senderLocked, err = database.IsLocked(s.Database.DB, tx.Source)
		}

		// Lock the sender
		if err := database.SetLock(s.Database.DB, tx.Source); err != nil {
			return fmt.Errorf("failed to lock Sender %d: %v", tx.Source, err)
		}
		s.PrePrepare(request.Transaction, request.ActiveServersForSourceShard, request.ByzantineServersForSourceShard)
		if err := database.UnsetLock(s.Database.DB, tx.Source); err != nil {
			return fmt.Errorf("failed to unlock Sender %d: %v", tx.Source, err)
		}
	}
	*reply = "success"
	// fmt.Printf("[DEBUG] Cross-shard transaction handled successfully on server %s\n", s.ID)
	return nil
}
func (s *Server) Handle2PCPrepare(request struct {
	CommittedTransactions           []shared.Transaction
	ClientTransaction               shared.Transaction
	ActiveServers                   []string
	ByzantineServers                []string
	ClientPublicKey                 *rsa.PublicKey
	CommittedTransactionsPublicKeys []*rsa.PublicKey
}, reply *string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx := request.ClientTransaction
	publicKey := request.ClientPublicKey
	// check if the clientTransaction is valid
	if !s.verifyPublicKey(tx, publicKey, reply) {
		*reply = "ack"             // Set reply to "ack" for invalid public key
		return fmt.Errorf("abort") // Return "abort"
	}
	//check if the committedTransactions are valid
	for i, committedTx := range request.CommittedTransactions {
		if !s.verifyPublicKey(committedTx, request.CommittedTransactionsPublicKeys[i], reply) {
			*reply = "ack"             // Set reply to "ack" for invalid committed transaction
			return fmt.Errorf("abort") // Return "abort"
		}
	}
	// check if the clientTransaction and CommittedTransactions are matching
	for _, committedTx := range request.CommittedTransactions {
		if committedTx.TransactionID != tx.TransactionID {
			*reply = "ack"             // Set reply to "ack" for transaction mismatch
			return fmt.Errorf("abort") // Return "abort"
		}
	}
	// if its the leader then initiate pbft in the cluster
	if s.Name == shared.GetClusterLeaderId(s.Name) {

		// Check if receiver is locked
		receiverLocked, err := database.IsLocked(s.Database.DB, tx.Destination)
		for err != nil || receiverLocked {
			// Wait for the lock to be released
			time.Sleep(100 * time.Millisecond)
			receiverLocked, err = database.IsLocked(s.Database.DB, tx.Destination)
		}

		// Lock the receiver
		if err := database.SetLock(s.Database.DB, tx.Destination); err != nil {
			*reply = "ack"             // Set reply to "ack" for lock failure
			return fmt.Errorf("abort") // Return "abort"
		}
		tx.Status = "pre-prepare"
		s.MaxSequenceNumber++
		tx.SequenceNumber = s.MaxSequenceNumber
		s.Transactions = append(s.Transactions, tx)
		signedTx, err := s.signTransaction(tx)
		if err != nil {
			*reply = "ack" // Set reply to "ack" for signing failure
			fmt.Printf("returning nill at client-request sign")
			return fmt.Errorf("abort") // Return "abort"
		}
		tx.Signature = signedTx
		s.PrePrepare(tx, request.ActiveServers, request.ByzantineServers)
		*reply = "success" // Set reply to "success" for successful consensus
		return nil
	}
	*reply = "ack" // Default reply if not the leader
	return nil     // Return "abort"
}

func (s *Server) Handle2PCCommit(args shared.TwoPCArgs, reply *shared.TwoPCReply) error {
	// Lock the database for thread safety
	s.mu.Lock()
	defer s.mu.Unlock()

	// fmt.Printf("[DEBUG] Handling 2PC commit on server %s with role %s and crossShardRole %s\n", s.ID, args.Role, args.CrossShardRole)
	tx := args.Transaction
	// If the transaction exists, handle based on the role
	if args.Role == "2PCcommit" {
		// fmt.Printf("[DEBUG] Received 2PC commit for transaction %s\n", tx.TransactionID)
		// Add transaction to the database and change its status to "C"
		if err := database.RemoveFromWAL(s.Database.DB, tx.TransactionID, tx.Source, tx.Destination, tx.Amount, tx.SequenceNumber); err != nil {
			return fmt.Errorf("failed to remove transaction %s from WAL: %v", tx.TransactionID, err)
		}
		// add status C
		if err := database.AddTransaction(s.Database.DB, uuid.New().String(), tx.Source, tx.Destination, tx.Amount, tx.SequenceNumber, tx.ContactServer, "C"); err != nil {
			return fmt.Errorf("failed to add transaction %s: %v", tx.TransactionID, err)
		}
		// unset the lock of destination shard
		if err := database.UnsetLock(s.Database.DB, tx.Destination); err != nil {
			return fmt.Errorf("failed to unlock Receiver %d: %v", tx.Destination, err)
		}
	} else if args.Role == "2PCabort" {
		// Undo the operation
		// fmt.Printf("[DEBUG] Received 2PC abort for transaction %s\n", tx.TransactionID)
		walDetails, err := database.GetWALDetails(s.Database.DB, tx.TransactionID)
		if err != nil {
			return fmt.Errorf("failed to get WAL details for transaction %s: %v", tx.TransactionID, err)
		}
		// Use walDetails as needed
		if err := database.RemoveFromWAL(s.Database.DB, walDetails.TransactionID, walDetails.Source, walDetails.Destination, walDetails.Amount, walDetails.SequenceNumber); err != nil {
			return fmt.Errorf("failed to remove transaction %s from WAL: %v", tx.TransactionID, err)
		}
		// undo the operation
		if err := database.UpdateClientBalance(s.Database.DB, walDetails.Destination, -walDetails.Amount); err != nil {
			return fmt.Errorf("failed to update balance for Receiver %d: %v", tx.Destination, err)
		}
		// Unset locks
		if err := database.UnsetLock(s.Database.DB, tx.Destination); err != nil {
			return fmt.Errorf("failed to unlock Receiver %d: %v", tx.Destination, err)
		}
	}

	return nil
}

// server.go

func (s *Server) GetBalance(clientID int, reply *int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	balance, err := database.GetClientBalance(s.Database.DB, clientID)
	if err != nil {
		return fmt.Errorf("failed to get balance for client %d: %v", clientID, err)
	}
	*reply = balance
	return nil
}

// verifyPublicKey verifies the signature of a given transaction
func (s *Server) verifyPublicKey(tx shared.Transaction, publicKey *rsa.PublicKey, reply *string) bool {
	// Create a hash of the transaction to verify
	txHash := sha256.Sum256([]byte(fmt.Sprintf("%d%d%d", tx.Source, tx.Destination, tx.Amount)))

	// Verify the signature using the public key
	err := rsa.VerifyPKCS1v15(publicKey, crypto.SHA256, txHash[:], tx.Signature)
	if err != nil {
		*reply = "invalid signature"
		return false
	}
	return true
}
func (s *Server) signTransaction(tx shared.Transaction) ([]byte, error) {
	txHash := sha256.Sum256([]byte(fmt.Sprintf("%d%d%d", tx.Source, tx.Destination, tx.Amount)))
	signedTx, err := rsa.SignPKCS1v15(rand.Reader, s.PrivateKey, crypto.SHA256, txHash[:])
	if err != nil {
		fmt.Printf("Error signing the transaction on server %s: %v\n", s.Name, err)
		return nil, err
	}
	return signedTx, nil
}

// PrePrepare sends the transaction to all the remaining servers in the active servers list excpet the leader server // sending from leader
func (s *Server) PrePrepare(tx shared.Transaction, activeServerList []string, byzantineServerList []string) {
	for _, serverName := range activeServerList {
		if serverName != s.Name {
			serverAddr, ok := shared.ServerAddresses(s.Name)
			if ok != nil {
				continue
			}
			client, err := rpc.Dial("tcp", serverAddr)
			if err != nil {
				fmt.Printf("Error connecting to server %s: %v\n", serverName, err)
				continue
			}
			defer client.Close()
			request := TransactionRequest{
				Transaction:      tx,
				ActiveServers:    activeServerList,
				ByzantineServers: byzantineServerList,
				PublicKey:        s.PublicKey,
			}
			var reply string
			err = client.Call(fmt.Sprintf("Server.%s.HandleTransaction", serverName), request, &reply)
			if err != nil {
				fmt.Printf("Error sending transaction to server %s: %v\n", serverName, err)
			} else {
				// fmt.Printf("Sending Pre-Prepare from %s to %s tx: %s->%s:%d sqno: %d\n", s.Name, serverName, tx.Source, tx.Destination, tx.Amount, tx.SequenceNumber)
			}
		}
	}
}
func (s *Server) sendFromPrimaryWithThresholdKey(tx shared.Transaction, activeServerList []string, byzantineServerList []string, thresholdPublicKey *rsa.PublicKey) {
	for _, serverName := range activeServerList {
		if serverName != s.Name {
			serverAddr, ok := shared.ServerAddresses(s.Name)
			if ok != nil {
				// fmt.Printf("Server address not found for server %s\n", serverID)
				continue
			}
			client, err := rpc.Dial("tcp", serverAddr)
			if err != nil {
				fmt.Printf("Error connecting to server %s: %v\n", serverName, err)
				fmt.Printf("Threshold signature: %x\n", thresholdPublicKey) // Added for debugging
				continue
			}
			defer client.Close()
			request := TransactionRequest{
				Transaction:      tx,
				ActiveServers:    activeServerList,
				ByzantineServers: byzantineServerList,
				PublicKey:        s.PublicKey,
			}
			var reply string
			err = client.Call(fmt.Sprintf("Server.%s.HandleTransaction", serverName), request, &reply)
			if err != nil {
				fmt.Printf("Error sending prepare transaction to server %s: %v\n", serverName, err)
			} else {
				// fmt.Printf("Prepare transaction sent to server %s, reply: %s\n", serverName, reply)
			}
		}
	}
}

// sendPrepareToPrimary sends the transaction to the primary server during the prepare phase
func (s *Server) sendFromBackupsToPrimary(tx shared.Transaction, activeServerList []string, byzantineServerList []string) {
	serverAddr, ok := shared.ServerAddresses(shared.GetClusterLeaderId(s.Name))
	if ok != nil {
		return
	}
	client, err := rpc.Dial("tcp", serverAddr)
	if err != nil {
		return
	}
	defer client.Close()
	request := TransactionRequest{
		Transaction:      tx,
		ActiveServers:    activeServerList,
		ByzantineServers: byzantineServerList,
		PublicKey:        s.PublicKey,
	}
	var prepareReply string
	err = client.Call(fmt.Sprintf("Server.%s.HandleTransaction", shared.GetClusterLeaderId(s.Name)), request, &prepareReply)
	if err != nil {
		return
	}
}
func (s *Server) send2PCPrepare(clientTransaction shared.Transaction, committedTransactionList []shared.Transaction, activeServerList []string, byzantineServerList []string) {
	for _, serverName := range activeServerList {
		if serverName != s.Name {
			serverAddr, ok := shared.ServerAddresses(s.Name)
			if ok != nil {
				continue
			}
			client, err := rpc.Dial("tcp", serverAddr)
			if err != nil {
				fmt.Printf("Error connecting to server %s: %v\n", serverName, err)
				continue
			}
			defer client.Close()
			request := TwoPCPrepareRequest{
				CommittedTransactions: committedTransactionList,
				ClientTransaction:     clientTransaction,
				ActiveServers:         activeServerList,
				ByzantineServers:      byzantineServerList,
				PublicKey:             s.PublicKey,
			}
			var reply string
			err = client.Call(fmt.Sprintf("Server.%s.Handle2PCPrepare", serverName), request, &reply)
			if err != nil {
				fmt.Printf("Error sending transaction to server %s: %v\n", serverName, err)
			} else {
				// fmt.Printf("Sending Pre-Prepare from %s to %s tx: %s->%s:%d sqno: %d\n", s.Name, serverName, tx.Source, tx.Destination, tx.Amount, tx.SequenceNumber)
			}
		}
	}
}
