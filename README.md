# Distributed Banking System with Byzantine Fault Tolerance

A distributed banking system implemented in Go that provides Byzantine fault tolerance (BFT) using the Practical Byzantine Fault Tolerance (PBFT) consensus algorithm and Two-Phase Commit (2PC) protocol for cross-shard transactions.

## Table of Contents

- [Overview](#overview)
- [Architecture](#architecture)
- [Features](#features)
- [System Components](#system-components)
- [Consensus Protocols](#consensus-protocols)
- [Transaction Types](#transaction-types)
- [Database Schema](#database-schema)
- [Installation](#installation)
- [Usage](#usage)
- [Project Structure](#project-structure)
- [Key Algorithms](#key-algorithms)
- [Security Features](#security-features)
- [Performance Metrics](#performance-metrics)
- [Bonus Features](#bonus-features)
- [Testing](#testing)
- [Limitations and Future Work](#limitations-and-future-work)

## Overview

This project implements a distributed banking system that can handle transactions across multiple sharded clusters while maintaining consistency and fault tolerance. The system is designed to:

- Handle both intra-shard and cross-shard transactions
- Tolerate Byzantine failures (malicious or faulty nodes)
- Maintain data consistency across distributed clusters
- Provide cryptographic security through digital signatures
- Support dynamic cluster configuration

## Architecture

The system follows a **sharded cluster architecture**:

```
┌─────────────────────────────────────────────────────────┐
│                    Client Layer                         │
│  (Generates transactions, signs with RSA keys)          │
└─────────────────────────────────────────────────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────────┐
│              Cluster Layer (Multiple Clusters)          │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐             │
│  │ Cluster 1│  │ Cluster 2│  │ Cluster N│             │
│  │  (C1)    │  │  (C2)    │  │  (CN)    │             │
│  └──────────┘  └──────────┘  └──────────┘             │
│       │             │             │                     │
│  ┌────┴────┐   ┌────┴────┐   ┌────┴────┐              │
│  │ S1-S4   │   │ S5-S8   │   │ S9-S12  │              │
│  │(Servers)│   │(Servers)│   │(Servers)│              │
│  └─────────┘   └─────────┘   └─────────┘              │
└─────────────────────────────────────────────────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────────┐
│              Database Layer (SQLite per Server)          │
│  - Clients table (balance, locks)                      │
│  - Transactions table                                   │
│  - WAL (Write-Ahead Log) for cross-shard transactions  │
└─────────────────────────────────────────────────────────┘
```

### Key Concepts

- **Clusters**: Groups of servers that maintain replicas of a shard
- **Shards**: Logical partitions of data (e.g., client IDs 1-1000, 1001-2000, etc.)
- **Servers**: Individual nodes within a cluster (e.g., S1, S2, S3, S4)
- **Leaders**: Primary servers in each cluster (e.g., S1 for cluster 1, S5 for cluster 2)

## Features

### Core Features

1. **Byzantine Fault Tolerance (BFT)**
   - Implements PBFT consensus algorithm
   - Tolerates up to ⌊(n-1)/3⌋ Byzantine failures in a cluster of n servers
   - Uses threshold signatures via Shamir's Secret Sharing

2. **Two-Phase Commit (2PC)**
   - Handles cross-shard transactions atomically
   - Coordinator-participant model
   - Write-Ahead Log (WAL) for recovery

3. **Transaction Processing**
   - Intra-shard transactions (within same cluster)
   - Cross-shard transactions (between different clusters)
   - Automatic retry mechanism for failed transactions
   - Transaction sequencing and ordering

4. **Security**
   - RSA-2048 digital signatures for transaction authentication
   - Public key verification at each consensus phase
   - Threshold signatures for quorum decisions

5. **Data Consistency**
   - Client locking mechanism to prevent concurrent modifications
   - Sequential execution of committed transactions
   - ACID properties for transaction execution

## System Components

### 1. Main Entry Point (`main.go`)

The main orchestrator that:
- Configures cluster topology (number of clusters, servers per cluster)
- Initializes server-to-cluster mappings
- Assigns data shards to clusters
- Starts all server RPC services
- Parses CSV transaction sets
- Processes transactions interactively
- Provides CLI for balance queries, datastore inspection, and performance metrics

**Key Functions:**
- `ConfigureClusters()`: Interactive cluster configuration
- `InitializeClusters()`: Creates cluster-server mappings
- `AssignShardsToClusters()`: Distributes data shards across clusters
- `PrintBalance()`: Query client balance across all servers in a cluster
- `PrintDatastore()`: Display all committed transactions per server
- `PrintPerformance()`: Show throughput and latency metrics

### 2. Server Module (`server/server.go`)

The core server implementation that handles:
- Transaction processing through PBFT phases
- Cross-shard transaction coordination
- 2PC protocol execution
- Database operations
- Cryptographic operations (signing, verification)

**PBFT Phases:**
1. **Client-Request**: Initial transaction submission
2. **Pre-Prepare**: Leader assigns sequence number and broadcasts
3. **Prepare**: Backup servers validate and send prepare messages
4. **Prepared**: Threshold signature combination (quorum reached)
5. **Commit**: Servers commit and execute transaction
6. **Committed**: Final state broadcast

**Key Methods:**
- `HandleTransaction()`: Main transaction handler for PBFT phases
- `HandleClientCrossShardTransaction()`: Cross-shard transaction coordinator
- `Handle2PCPrepare()`: 2PC prepare phase handler
- `Handle2PCCommit()`: 2PC commit/abort handler
- `PrePrepare()`: Broadcast pre-prepare messages
- `sendFromBackupsToPrimary()`: Send prepare/commit messages to leader
- `sendFromPrimaryWithThresholdKey()`: Broadcast threshold-signed messages

### 3. Client Module (`client/client.go`)

Client-side transaction submission:
- Generates RSA key pairs for signing
- Signs transactions with SHA-256 hash
- Sends transactions to appropriate servers
- Handles intra-shard and cross-shard transaction routing

**Key Functions:**
- `SendIntraShardTransaction()`: Submit transaction within a shard
- `SendCrossShardTransaction()`: Submit cross-shard transaction
- `Send2PCCommit()`: Send 2PC commit/abort decision

### 4. Database Module (`database/`)

SQLite-based persistence layer:

**Files:**
- `database.go`: Database initialization and schema creation
- `util.go`: CRUD operations for clients, transactions, and WAL
- `db_smallbank.go`: SmallBank benchmark schema (bonus feature)

**Tables:**
1. **clients**: Client balances and locks
   ```sql
   CREATE TABLE clients (
       client_id INTEGER PRIMARY KEY,
       balance INTEGER NOT NULL,
       lock BOOLEAN NOT NULL DEFAULT 0
   );
   ```

2. **transactions**: Transaction history
   ```sql
   CREATE TABLE transactions (
       transaction_id TEXT PRIMARY KEY,
       source INTEGER NOT NULL,
       destination INTEGER NOT NULL,
       amount INTEGER NOT NULL,
       sequence_number INTEGER NOT NULL,
       contact_server INTEGER NOT NULL,
       status TEXT NOT NULL,
       created_at DATETIME DEFAULT CURRENT_TIMESTAMP
   );
   ```

3. **WAL**: Write-Ahead Log for cross-shard transactions
   ```sql
   CREATE TABLE WAL (
       transaction_id TEXT NOT NULL,
       source INTEGER NOT NULL,
       destination INTEGER NOT NULL,
       amount INTEGER NOT NULL,
       sequence_number INTEGER NOT NULL,
       PRIMARY KEY (transaction_id)
   );
   ```

**Key Functions:**
- `InitDatabase()`: Initialize database with schema
- `GetClientBalance()`: Read client balance
- `UpdateClientBalance()`: Modify client balance
- `SetLock()`/`UnsetLock()`: Client locking mechanism
- `AddTransaction()`: Record committed transaction
- `AddToWAL()`: Log cross-shard transaction
- `GetAllTransactions()`: Retrieve transaction history

### 5. Shared Utilities (`shared/`)

Common utilities and data structures:

**Key Files:**
- `types.go`: Transaction and 2PC argument structures
- `shared.go`: Quorum size calculation
- `transaction.go`: Transaction data structure
- `serveraddress.go`: Dynamic server address resolution (localhost:5000+)
- `getclusterleadername.go`: Leader election logic
- `servertoclusteridmap.go`: Server-to-cluster mapping
- `contactserverforclusterid.go`: Contact server selection
- `istransactioncrossshard.go`: Cross-shard detection
- `isthisbyzantine.go`: Byzantine server identification

### 6. CSV Parser (`csv_parser/csv_parser.go`)

Parses test transaction sets from CSV files:
- Extracts transaction sets with metadata
- Parses active server lists
- Parses contact server lists
- Parses Byzantine server lists
- Handles transaction tuples (source, destination, amount)

**CSV Format:**
```
SetNumber,Transaction,ActiveServers,ContactServers,ByzantineServers
1,(1,2,5),["S1","S2","S3"],["S1","S2"],["S3"]
  ,(3,4,10),...
```

### 7. Bonus Features (`bonus/`)

1. **SmallBank Benchmark** (`smallbankbenchmark.go`)
   - Implements SmallBank transaction types:
     - `Balance`: Get total balance (checking + savings)
     - `DepositChecking`: Add to checking account
     - `TransactSavings`: Modify savings account
     - `Amalgamate`: Transfer all savings to checking
     - `WriteCheck`: Withdraw from checking (allows negative)
     - `SendPayment`: Transfer between checking accounts

2. **Multi-Destination Transactions** (`multidestination.go`)
   - Support for transactions with multiple recipients
   - Validates total amount against source balance
   - Locks all participants (source + all destinations)
   - Atomic commit across all destinations

## Consensus Protocols

### PBFT (Practical Byzantine Fault Tolerance)

**Assumptions:**
- Asynchronous network (messages may be delayed but eventually delivered)
- Up to ⌊(n-1)/3⌋ Byzantine failures in a cluster of n servers
- Cryptographic signatures prevent message forgery

**Phases:**

1. **Request**: Client sends transaction to leader
2. **Pre-Prepare**: Leader assigns sequence number, signs, and broadcasts
3. **Prepare**: Backup servers validate and send prepare messages to leader
4. **Prepared**: Leader combines signatures (threshold signature) when quorum reached
5. **Commit**: Leader broadcasts commit decision, servers commit locally
6. **Reply**: Servers execute and respond to client

**Quorum Calculation:**
- Quorum size = ⌊(n+1)/2⌋ where n = servers per cluster
- At least 2f+1 servers needed to tolerate f Byzantine failures

### Two-Phase Commit (2PC)

Used for **cross-shard transactions**:

**Phase 1: Prepare**
1. Coordinator (source shard leader) initiates PBFT to commit transaction locally
2. Coordinator sends 2PC prepare to participant (destination shard)
3. Participant validates transaction and committed transactions list
4. Participant runs PBFT to prepare transaction
5. Participant responds with "ack" (ready) or "abort"

**Phase 2: Commit/Abort**
1. Coordinator receives responses from all participants
2. If all "ack": Coordinator runs PBFT to decide "commit"
3. If any "abort": Coordinator runs PBFT to decide "abort"
4. Coordinator sends commit/abort decision to participants
5. Participants execute decision:
   - **Commit**: Remove from WAL, add to transactions, unlock
   - **Abort**: Remove from WAL, undo balance change, unlock

**Write-Ahead Log (WAL):**
- Cross-shard transactions are logged to WAL before commit
- Enables recovery if coordinator fails
- Removed on commit or abort

## Transaction Types

### Intra-Shard Transactions

Transactions where source and destination belong to the same shard/cluster.

**Flow:**
1. Client sends to contact server (leader)
2. Leader initiates PBFT
3. Transaction commits after PBFT consensus
4. Locks released immediately after commit

**Example:** Client 1 → Client 2 (both in shard 1-1000)

### Cross-Shard Transactions

Transactions where source and destination belong to different shards/clusters.

**Flow:**
1. Client sends to source shard coordinator
2. Source shard runs PBFT to commit locally
3. Source shard sends 2PC prepare to destination shard
4. Destination shard validates and prepares (PBFT)
5. Source shard decides commit/abort (PBFT)
6. Destination shard executes decision

**Example:** Client 1 (shard 1) → Client 2000 (shard 2)

## Database Schema

### Clients Table
- `client_id`: Primary key (1-3000)
- `balance`: Current balance (initialized to 10)
- `lock`: Boolean flag for concurrency control

### Transactions Table
- `transaction_id`: UUID primary key
- `source`: Source client ID
- `destination`: Destination client ID
- `amount`: Transaction amount
- `sequence_number`: PBFT sequence number
- `contact_server`: Server index that received transaction
- `status`: Transaction status (e.g., "C" for committed, "P" for prepared)
- `created_at`: Timestamp

### WAL Table
- `transaction_id`: Primary key
- `source`: Source client ID
- `destination`: Destination client ID
- `amount`: Transaction amount
- `sequence_number`: Sequence number

## Installation

### Prerequisites

- Go 1.23.2 or later
- SQLite3 (via `github.com/mattn/go-sqlite3`)

### Dependencies

The project uses the following Go modules:
- `github.com/google/uuid`: UUID generation
- `github.com/mattn/go-sqlite3`: SQLite driver
- `github.com/hashicorp/vault/shamir`: Shamir's Secret Sharing for threshold signatures

### Setup

1. **Clone the repository:**
   ```bash
   git clone <repository-url>
   cd 2pcbyz-saicharanjakkula
   ```

2. **Navigate to the distributed-banking directory:**
   ```bash
   cd distributed-banking
   ```

3. **Install dependencies:**
   ```bash
   go mod download
   ```

4. **Build the project:**
   ```bash
   go build -o banking-system main.go
   ```

## Usage

### Running the System

1. **Start the system:**
   ```bash
   go run main.go
   # or
   ./banking-system
   ```

2. **Configure clusters:**
   ```
   Enter the number of clusters: 3
   Enter the number of servers per cluster: 4
   ```

   This creates:
   - 3 clusters (C1, C2, C3)
   - 4 servers per cluster (S1-S4, S5-S8, S9-S12)
   - Total: 12 servers

3. **System processes transactions from CSV:**
   - Reads `Lab4_Testset_1.csv` or `Lab4_Testset_2.csv`
   - Processes each transaction set
   - Displays active servers, Byzantine servers, and contact servers

4. **Interactive Menu (after each set):**
   ```
   Select an option:
   1 - Proceed to next set
   2 - Print balance
   3 - Print Datastore
   4 - Print Performance
   ```

### Example Session

```
Starting Distributed Banking System...
Processing Set 1
Active Servers: [S1 S2 S3 S4]
Byzantine Servers: [S3]
Contact Servers: [S1 S2]
Transactions:
    Transaction: 1 -> 2, Amount: 5
    Transaction: 3 -> 4, Amount: 10
Sending intra-shard transaction to S1
received prepare at S2 tx: 1->2:5 sqno: 1
received prepared at S1 tx: 1->2:5 sqno: 1
received commit at S1 tx: 1->2:5 sqno: 1
Select an option:
1 - Proceed to next set
2 - Print balance
3 - Print Datastore
4 - Print Performance
```

### Querying Balance

Select option 2 and enter a client ID:
```
Enter client ID to get balance: 1
Server | 1
S1     | 5
S2     | 5
S3     | 5
S4     | 5
```

### Viewing Datastore

Select option 3 to see all committed transactions:
```
S1 : -> |<1,1>, C,(1,2,5)| -> |<2,1>, C,(3,4,10)|
S2 : -> |<1,1>, C,(1,2,5)| -> |<2,1>, C,(3,4,10)|
...
```

### Performance Metrics

Select option 4 to view:
```
Total Transactions: 50
Total Time: 2.5s
Average Latency per Transaction: 50ms
Throughput: 20.00 transactions per second
```

## Project Structure

```
distributed-banking/
├── main.go                    # Main entry point and orchestration
├── go.mod                     # Go module dependencies
├── go.sum                     # Dependency checksums
├── Lab4_Testset_1.csv         # Test transaction set 1
├── Lab4_Testset_2.csv         # Test transaction set 2
├── db_S*.db                   # SQLite database files (one per server)
│
├── client/
│   └── client.go              # Client-side transaction submission
│
├── server/
│   └── server.go              # Server implementation (PBFT, 2PC)
│
├── database/
│   ├── database.go            # Database initialization and schema
│   ├── util.go                # Database operations (CRUD)
│   └── db_smallbank.go        # SmallBank schema (bonus)
│
├── shared/
│   ├── types.go               # Common data structures
│   ├── shared.go              # Quorum calculations
│   ├── transaction.go         # Transaction type definition
│   ├── serveraddress.go       # Server address resolution
│   ├── getclusterleadername.go # Leader election
│   ├── servertoclusteridmap.go # Server-cluster mapping
│   ├── contactserverforclusterid.go # Contact server selection
│   ├── istransactioncrossshard.go # Cross-shard detection
│   ├── isthisbyzantine.go     # Byzantine identification
│   ├── prepreparerequest.go   # Pre-prepare utilities
│   ├── filteractiveservers.go # Server filtering
│   ├── caluculatenewshards.go # Shard calculation
│   ├── serverstatus.go        # Server status utilities
│   ├── transactionqueue.go    # Transaction queue management
│   └── ...                    # Additional utilities
│
├── csv_parser/
│   └── csv_parser.go          # CSV transaction set parser
│
└── bonus/
    ├── smallbankbenchmark.go  # SmallBank benchmark implementation
    └── multidestination.go    # Multi-destination transaction support
```

## Key Algorithms

### 1. Shard Assignment

Shards are distributed evenly across clusters:
```go
itemsPerCluster = dataCount / clusterCount
clusterID = (shardID - 1) / itemsPerCluster + 1
```

Example: 3000 clients, 3 clusters → 1000 clients per cluster

### 2. Leader Election

Leaders are statically assigned based on server number:
- Cluster 1 (S1-S4): Leader = S1
- Cluster 2 (S5-S8): Leader = S5
- Cluster 3 (S9-S12): Leader = S9

### 3. Quorum Calculation

```go
quorumSize = (serversPerCluster + 1) / 2
```

For 4 servers per cluster: quorum = 2 (need 2 prepare messages)

### 4. Threshold Signatures

Uses Shamir's Secret Sharing to combine signatures:
- Each server signs with its private key
- Leader combines signatures when quorum reached
- Combined signature represents quorum decision

### 5. Transaction Sequencing

Transactions are assigned sequence numbers by the leader:
- Sequence numbers are monotonically increasing
- Transactions execute in sequence number order
- Ensures total ordering across all servers

## Security Features

1. **Digital Signatures**
   - RSA-2048 key pairs per server
   - SHA-256 hashing for transaction content
   - PKCS1v15 signature scheme

2. **Public Key Verification**
   - Each message phase verifies sender's signature
   - Prevents message forgery and tampering
   - Byzantine servers cannot forge valid signatures

3. **Threshold Signatures**
   - Quorum of signatures required for decisions
   - Single Byzantine server cannot create valid threshold signature
   - Shamir's Secret Sharing for combination

4. **Client Locking**
   - Prevents concurrent modifications
   - Deadlock prevention through ordered locking
   - Automatic unlock after transaction commit/abort

## Performance Metrics

The system tracks:
- **Total Transactions**: Number of successfully processed transactions
- **Total Time**: Cumulative processing time
- **Average Latency**: Mean time per transaction
- **Throughput**: Transactions per second

### Factors Affecting Performance

1. **Network Latency**
   - RPC calls between servers add network overhead
   - Multiple phases in PBFT increase latency

2. **Byzantine Failures**
   - Byzantine servers may delay or drop messages
   - Retry mechanisms add additional latency

3. **Cross-Shard Transactions**
   - 2PC protocol adds extra round-trips
   - WAL operations add I/O overhead

4. **Lock Contention**
   - Waiting for locks increases transaction time
   - High contention reduces throughput

## Bonus Features

### SmallBank Benchmark

Implements the SmallBank benchmark with 6 transaction types:
- Useful for performance evaluation
- More complex than simple balance transfers
- Supports checking and savings accounts

### Multi-Destination Transactions

Allows a single transaction to have multiple recipients:
- Validates total amount against source balance
- Locks all participants atomically
- Commits to all destinations in one transaction

## Testing

### Test Files

- `Lab4_Testset_1.csv`: First test set
- `Lab4_Testset_2.csv`: Second test set

### Test Scenarios

1. **Intra-Shard Transactions**
   - Same shard source and destination
   - Should complete with PBFT only

2. **Cross-Shard Transactions**
   - Different shard source and destination
   - Should complete with PBFT + 2PC

3. **Byzantine Failures**
   - Some servers marked as Byzantine
   - System should tolerate failures and still reach consensus

4. **Concurrent Transactions**
   - Multiple transactions on same clients
   - Locking should prevent conflicts

### Manual Testing

1. Run the system with different cluster configurations
2. Process transaction sets from CSV files
3. Verify balances after transactions
4. Check datastore for committed transactions
5. Monitor performance metrics

## Limitations and Future Work

### Current Limitations

1. **Static Leader Assignment**
   - Leaders are hardcoded, no dynamic leader election
   - Leader failure requires manual intervention

2. **No View Change**
   - PBFT view change not implemented
   - Cannot recover from leader failures automatically

3. **Limited Error Handling**
   - Some error cases may not be fully handled
   - Network partitions not explicitly handled

4. **Synchronous Execution**
   - Transactions execute sequentially
   - No parallel execution optimization

5. **Missing WAL Functions**
   - `RemoveFromWAL` and `GetWALDetails` referenced but not fully implemented
   - May cause issues with 2PC abort recovery

### Future Enhancements

1. **Dynamic Leader Election**
   - Implement view change protocol
   - Automatic leader re-election on failure

2. **Checkpointing**
   - Periodic state snapshots
   - Reduce log size and recovery time

3. **Parallel Execution**
   - Execute non-conflicting transactions in parallel
   - Improve throughput

4. **Network Partition Handling**
   - Detect and handle network partitions
   - Graceful degradation

5. **Enhanced Monitoring**
   - Real-time metrics dashboard
   - Detailed logging and tracing

6. **Load Balancing**
   - Distribute load across servers
   - Adaptive shard rebalancing

## Contributing

This project was developed as part of a distributed systems course/lab assignment. For questions or improvements, please refer to the course materials or contact the maintainers.

## License

[Specify license if applicable]

## Acknowledgments

- PBFT algorithm: Castro and Liskov (1999)
- Shamir's Secret Sharing: Shamir (1979)
- SmallBank Benchmark: Alomari et al. (2008)

---

**Author:** Sai Charan Jakkula  
**Project:** Distributed Banking System with 2PC and Byzantine Fault Tolerance
