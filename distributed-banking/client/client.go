package client

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"distributed-banking/shared"
	"fmt"
	"net/rpc"
	"time"
)

var privateKey *rsa.PrivateKey
var publicKey *rsa.PublicKey

func init() {
	var err error
	// Generate an RSA private key for signing transactions
	privateKey, err = rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		fmt.Println("Error generating RSA private key:", err)
	}
	// Extract the public key from the private key
	publicKey = &privateKey.PublicKey
}

func ConnectToServer(serverName string) *rpc.Client {
	serverAddress, ok := shared.ServerAddresses(serverName)
	if ok != nil {
		// fmt.Printf("Server address not found for server %s\n", serverName)
	}
	if serverAddress == "" {
		// fmt.Printf("Error: Server address for %s is missing\n", serverName)
		return nil
	}

	client, err := rpc.Dial("tcp", serverAddress)
	if err != nil {
		// fmt.Printf("Error connecting to server %s at %s: %v\n", serverName, serverAddress, err)
		return nil
	}
	return client
}

// client.go

func SendIntraShardTransaction(serverName string, tx shared.Transaction, activeServers []string, byzantineServers []string) (bool, time.Duration) {
	startTime := time.Now()
	// Create a hash of the transaction to sign
	txHash := sha256.Sum256([]byte(fmt.Sprintf("%d%d%d", tx.Source, tx.Destination, tx.Amount)))

	// Sign the hash using the RSA private key
	signature, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, txHash[:])
	if err != nil {
		fmt.Println("Error signing the transaction:", err)
		return false, time.Since(startTime)
	}
	tx.Signature = signature
	client := ConnectToServer(serverName)
	if client == nil {
		// fmt.Println("Failed to connect to server, transaction aborted.")
		return false, time.Since(startTime)
	}
	defer client.Close()

	type TransactionRequest struct {
		Transaction      shared.Transaction
		ActiveServers    []string
		ByzantineServers []string
		PublicKey        *rsa.PublicKey
	}

	request := TransactionRequest{
		Transaction:      tx,
		ActiveServers:    activeServers,
		ByzantineServers: byzantineServers,
		PublicKey:        publicKey,
	}

	var reply string
	err = client.Call(fmt.Sprintf("Server.%s.HandleTransaction", serverName), request, &reply)
	if err != nil {
		// fmt.Println("Transaction error:", err)
		return false, time.Since(startTime)
	}
	elapsedTime := time.Since(startTime)
	return true, elapsedTime
}

func SendCrossShardTransaction(serverName string, tx shared.Transaction, activeServers []string, role string) (bool, time.Duration) {
	startTime := time.Now()
	client := ConnectToServer(serverName)
	if client == nil {
		// fmt.Println("Failed to connect to server, transaction aborted.")
		return false, time.Since(startTime)
	}
	defer client.Close()

	type TransactionRequest struct {
		Transaction   shared.Transaction
		ActiveServers []string
		Role          string // Indicates whether the server is "source" or "destination"
	}

	request := TransactionRequest{
		Transaction:   tx,
		ActiveServers: activeServers,
		Role:          role,
	}

	var reply string
	err := client.Call(fmt.Sprintf("Server.%s.HandleCrossShardTransaction", serverName), request, &reply)
	if err != nil {
		// fmt.Printf("Cross-shard transaction error on server %s (%s): %v\n", serverName, role, err)
		return false, time.Since(startTime)
	}

	// fmt.Printf("Cross-shard transaction response from server %s (%s): %s\n", serverName, role, reply)
	elapsedTime := time.Since(startTime)
	return reply == "success", elapsedTime
}

// Send2PCCommit sends a 2PC commit/abort message to the specified server
func Send2PCCommit(serverID string, transaction shared.Transaction, role string, crossShardRole string) bool {
	// fmt.Printf("Debug: Sending 2PC commit to server %s with role %s and crossShardRole %s\n", serverID, role, crossShardRole)
	client := ConnectToServer(serverID)
	if client == nil {
		// fmt.Printf("Error connecting to server %s\n", serverID)
		return false
	}
	defer client.Close()

	// Prepare arguments for the RPC call
	args := shared.TwoPCArgs{
		Transaction:    transaction,
		Role:           role,
		CrossShardRole: crossShardRole,
	}
	var reply shared.TwoPCReply

	// Make the RPC call
	err := client.Call(fmt.Sprintf("Server.%s.Handle2PCCommit", serverID), args, &reply)
	if err != nil {
		// fmt.Printf("Error in 2PC call to server %s: %v\n", serverID, err)
		return false
	}

	return reply.Success
}
