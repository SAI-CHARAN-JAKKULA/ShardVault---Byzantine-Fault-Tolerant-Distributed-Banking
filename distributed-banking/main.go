package main

import (
	"bufio"
	"distributed-banking/client"
	"distributed-banking/csv_parser"
	"distributed-banking/server"
	"distributed-banking/shared"
	"fmt"
	"log"
	"net/rpc"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

type PendingCommit struct {
	sequence      int
	transaction   shared.Transaction
	sourceServers []string
	destServers   []string
	shouldCommit  bool
}

func main() {
	// Configure clusters dynamically
	clusterCount, serversPerCluster := ConfigureClusters()

	// Set the servers per cluster in shared package for quorum calculations
	shared.SetServersPerCluster(serversPerCluster)

	// Initialize clusters and servers
	clusterServers := InitializeClusters(clusterCount, serversPerCluster)
	fmt.Println("Clusters initialized:", clusterServers)

	// Initialize the server-to-cluster mapping
	shared.InitializeServerToClusterMapping(clusterServers)
	fmt.Println("Server-to-cluster mapping initialized.")

	// Assign data shards to clusters
	shardMapping := AssignShardsToClusters(3000, clusterCount)
	// fmt.Println("Shard mapping:", shardMapping)

	// Start servers using StartServerRPC
	// servers := []*server.Server{}
	for clusterID, serverIDs := range clusterServers {
		for _, serverID := range serverIDs {
			shardsForCluster := GetShardsForCluster(shardMapping, clusterID)
			port, ok := shared.ServerAddresses(serverID) // Retrieve port from shared.ServerAddresses
			if ok != nil {
				fmt.Printf("Server address not found for server %s\n", serverID)
				continue
			}
			srv := server.StartServerRPC(serverID, port, clusterID, shardsForCluster)
			if srv == nil {
				fmt.Printf("Error starting server %s\n", serverID)
				continue
			}
			// servers = append(servers, srv)
		}
	}
	fmt.Println("Debugging here")
	// Parse CSV for transaction sets
	sets, err := csv_parser.ParseCSV("Lab4_Testset_1.csv")
	if err != nil {
		log.Fatalf("Failed to parse CSV: %v", err)
	}
	fmt.Println("Starting Distributed Banking System...")
	var totalTransactions int
	var totalTime time.Duration
	// Process transactions
	reader := bufio.NewReader(os.Stdin)
	pendingCommits := []PendingCommit{}
	sequenceCounter := 0
	for _, set := range sets {
		fmt.Printf("Processing Set %d\n", set.SetNumber)
		fmt.Printf("Active Servers: %v\n", set.ActiveServerList)
		fmt.Printf("Byzantine Servers: %v\n", set.ByzantineServerList)
		fmt.Printf("Contact Servers: %v\n", set.ContactServerList)

		fmt.Println("Transactions:")

		for _, tx := range set.Transactions {
			fmt.Printf("    Transaction: %d -> %d, Amount: %d\n", tx.Source, tx.Destination, tx.Amount)
			clusterIDForSource := shardMapping[tx.Source]

			// Fetch the contact server using the cluster ID and set.ContactServerList
			contactServerForSource, err := shared.GetContactServerForCluster(clusterIDForSource, set.ContactServerList)
			if err != nil {
				fmt.Printf("Error fetching contact server for cluster %s: %v\n", clusterIDForSource, err)
				continue
			}
			clusterIDForDestination := shardMapping[tx.Destination]
			contactServerForDestination, err := shared.GetContactServerForCluster(clusterIDForDestination, set.ContactServerList)
			if err != nil {
				continue
			}
			activeServersForSourceShard := filterActiveServers(set.ActiveServerList, clusterIDForSource)

			transactionID := uuid.New().String()
			convertedTx := shared.Transaction{
				TransactionID: transactionID,
				Source:        tx.Source,
				Destination:   tx.Destination,
				Amount:        tx.Amount,
				ContactServer: 1,
				Status:        "client-request",
			}
			// Decide whether to send intra-shard or cross-shard transaction
			if contactServerForSource == contactServerForDestination {
				// Intra-shard transaction
				fmt.Printf("Sending intra-shard transaction to %s\n", contactServerForSource)
				serverIndex, err := extractServerIndex(contactServerForSource)
				if err != nil {
					// fmt.Printf("Error extracting server index: %v\n", err)
				}
				convertedTx.ContactServer = serverIndex
				success, latency := client.SendIntraShardTransaction(contactServerForSource, convertedTx, activeServersForSourceShard, set.ByzantineServerList)
				if success {
					totalTransactions++
					totalTime += latency
				} else {
					// Start a timer for retrying the transaction
					retryDuration := 500 * time.Millisecond // Set the retry duration
					retryTimer := time.NewTimer(retryDuration)
					defer retryTimer.Stop()

					replyCount := 0
					retryCount := 0
					maxRetries := 3 // Set the maximum number of retries

					for {
						select {
						case <-retryTimer.C:
							if replyCount < 2 && retryCount < maxRetries {
								// Retry sending the transaction
								success, latency = client.SendIntraShardTransaction(contactServerForSource, convertedTx, activeServersForSourceShard, set.ByzantineServerList)
								if success {
									totalTransactions++
									totalTime += latency
									break // Exit the retry loop on success
								}
								retryCount++
								retryTimer.Reset(retryDuration) // Reset the timer for the next retry
							} else {
								fmt.Println("Transaction failed after retries.")
								return // Exit if max retries reached or received enough replies
							}
						default:
							// Here you can handle receiving replies if needed
							// Increment replyCount based on received replies
						}
					}
				}
			} else {
				// Cross-shard transaction
				convertedTx.Status = "P"
				retryDuration := 500 * time.Millisecond // Set the retry duration for cross-shard
				retryTimer := time.NewTimer(retryDuration)
				defer retryTimer.Stop()

				success := false
				retryCount := 0
				maxRetries := 3 // Set the maximum number of retries

				for {
					success, _ = client.SendCrossShardTransaction(contactServerForSource, convertedTx, activeServersForSourceShard, "source")
					if success {
						break // Exit the retry loop on success
					}
					retryCount++
					if retryCount >= maxRetries {
						fmt.Println("Cross-shard transaction failed after retries.")
						break
					}
					retryTimer.Reset(retryDuration) // Reset the timer for the next retry
					<-retryTimer.C                  // Wait for the timer to expire
				}
				// Store this cross-shard transaction for later 2PC
				sequenceCounter++
			}
			time.Sleep(10 * time.Millisecond)
		}
		time.Sleep(100 * time.Millisecond)
		// Interactive user options
		for {
			fmt.Println("Select an option: ")
			fmt.Println("1 - Proceed to next set")
			fmt.Println("2 - Print balance")
			fmt.Println("3 - Print Datastore")
			fmt.Println("4 - Print Performance")

			input, _ := reader.ReadString('\n')
			input = strings.TrimSpace(input)
			choice, err := strconv.Atoi(input)
			if err != nil {
				fmt.Println("Invalid input. Please enter a number from 1 to 4.")
				continue
			}

			if choice == 1 {
				break
			} else if choice == 2 {
				PrintBalance(shardMapping, clusterServers)
			} else if choice == 3 {
				PrintDatastore(clusterServers)
			} else if choice == 4 {
				PrintPerformance(totalTransactions, totalTime)
			} else {
				fmt.Println("Invalid choice. Please enter 1 to 4.")
			}
		}
	}
}

func ConfigureClusters() (int, int) {
	var clusterCount, serversPerCluster int
	fmt.Println("Enter the number of clusters:")
	fmt.Scan(&clusterCount)
	fmt.Println("Enter the number of servers per cluster:")
	fmt.Scan(&serversPerCluster)
	return clusterCount, serversPerCluster
}

func GetShardsForCluster(shardMapping map[int]string, clusterID string) []int {
	shards := []int{}
	for shardID, assignedClusterID := range shardMapping {
		if assignedClusterID == clusterID {
			shards = append(shards, shardID)
		}
	}
	return shards
}

func filterActiveServers(activeServers []string, clusterID string) []string {
	var filteredServers []string
	for _, server := range activeServers {
		// Use the shared mapping to find the cluster of the server
		serverClusterID, err := shared.GetClusterID(server)
		if err == nil && serverClusterID == clusterID {
			filteredServers = append(filteredServers, server)
		} else if err != nil {
			fmt.Printf("Error fetching cluster ID for server %s: %v\n", server, err)
		}
	}
	return filteredServers
}

func InitializeClusters(clusterCount int, serversPerCluster int) map[string][]string {
	clusterServers := make(map[string][]string)

	for i := 1; i <= clusterCount; i++ {
		clusterID := fmt.Sprintf("C%d", i)
		for j := 1; j <= serversPerCluster; j++ {
			serverID := fmt.Sprintf("S%d", (i-1)*serversPerCluster+j)
			clusterServers[clusterID] = append(clusterServers[clusterID], serverID)
		}
	}

	return clusterServers
}

func AssignShardsToClusters(dataCount int, clusterCount int) map[int]string {
	shardMapping := make(map[int]string)
	itemsPerCluster := dataCount / clusterCount

	for i := 1; i <= dataCount; i++ {
		clusterID := fmt.Sprintf("C%d", (i-1)/itemsPerCluster+1)
		shardMapping[i] = clusterID
	}

	return shardMapping
}

func PrintBalance(shardMapping map[int]string, clusterServers map[string][]string) {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Enter client ID to get balance: ")
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)
	clientID, err := strconv.Atoi(input)
	if err != nil {
		fmt.Println("Invalid client ID")
		return
	}
	clusterID, ok := shardMapping[clientID]
	if !ok {
		fmt.Printf("Client ID %d not found in shard mapping\n", clientID)
		return
	}
	fmt.Printf("Server | %d\n", clientID)
	servers := clusterServers[clusterID]
	for _, serverID := range servers {
		serverAddress, err := shared.ServerAddresses(serverID)
		if err != nil {
			fmt.Printf("Error getting server address for %s: %v\n", serverID, err)
			continue
		}
		client, err := rpc.Dial("tcp", serverAddress)
		if err != nil {
			fmt.Printf("Error connecting to server %s: %v\n", serverID, err)
			continue
		}
		defer client.Close()
		var balance int
		err = client.Call(fmt.Sprintf("Server.%s.GetBalance", serverID), clientID, &balance)
		if err != nil {
			fmt.Printf("Error getting balance from server %s: %v\n", serverID, err)
			continue
		}
		fmt.Printf("%s     | %d\n", serverID, balance)
	}
}

func PrintDatastore(clusterServers map[string][]string) {
	for _, servers := range clusterServers {
		for _, serverID := range servers {
			serverAddress, err := shared.ServerAddresses(serverID)
			if err != nil {
				fmt.Printf("Error getting server address for %s: %v\n", serverID, err)
				continue
			}
			client, err := rpc.Dial("tcp", serverAddress)
			if err != nil {
				fmt.Printf("Error connecting to server %s: %v\n", serverID, err)
				continue
			}
			defer client.Close()
			var transactions []shared.Transaction
			err = client.Call(fmt.Sprintf("Server.%s.CommittedTransactionsInDB", serverID), struct{}{}, &transactions)
			if err != nil {
				fmt.Printf("Error getting transactions from server %s: %v\n", serverID, err)
				continue
			}
			fmt.Printf("%s :", serverID)
			for _, tx := range transactions {
				if tx.Status == "" {
					fmt.Printf(" -> |<%d,%d>,(%d,%d,%d)|", tx.SequenceNumber, tx.ContactServer, tx.Source, tx.Destination, tx.Amount)
				} else {
					fmt.Printf(" -> |<%d,%d>, %s,(%d,%d,%d)|", tx.SequenceNumber, tx.ContactServer, tx.Status, tx.Source, tx.Destination, tx.Amount)
				}
			}
			fmt.Print("\n")
			fmt.Print("\n")
		}
	}
}

func PrintPerformance(totalTransactions int, totalTime time.Duration) {
	if totalTransactions == 0 {
		fmt.Println("No transactions processed.")
		return
	}
	avgLatency := totalTime / time.Duration(totalTransactions)
	throughput := float64(totalTransactions) / totalTime.Seconds()
	fmt.Printf("Total Transactions: %d\n", totalTransactions)
	fmt.Printf("Total Time: %v\n", totalTime)
	fmt.Printf("Average Latency per Transaction: %v\n", avgLatency)
	fmt.Printf("Throughput: %.2f transactions per second\n", throughput)
}

// extractServerIndex extracts the numeric index from a server ID string
func extractServerIndex(serverID string) (int, error) {
	if !strings.HasPrefix(serverID, "S") {
		return 0, fmt.Errorf("invalid server ID format: %s", serverID)
	}
	return strconv.Atoi(serverID[1:]) // Convert "S1" -> 1
}
