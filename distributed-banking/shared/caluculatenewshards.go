package shared

import "fmt"

func CalculateNewShards(transactionHistory [][2]int, currentShardMapping map[int]string, clusterCount int) map[int]string {
	newMapping := make(map[int]string)
	clusterSize := len(currentShardMapping) / clusterCount

	// Start with current mapping
	for id, cluster := range currentShardMapping {
		newMapping[id] = cluster
	}

	// Group frequently accessed pairs
	for _, pair := range transactionHistory {
		newMapping[pair[0]] = newMapping[pair[1]] // Assign both items to the same shard
	}

	// Balance shards across clusters
	clusterIndex := 1
	count := 0
	for id := range newMapping {
		if count == clusterSize {
			clusterIndex++
			count = 0
		}
		newMapping[id] = fmt.Sprintf("C%d", clusterIndex)
		count++
	}

	return newMapping
}
