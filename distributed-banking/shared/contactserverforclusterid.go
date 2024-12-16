package shared

import (
	"fmt"
	"strconv"
	"strings"
)

// GetContactServerForCluster retrieves the corresponding server for the given cluster ID
func GetContactServerForCluster(clusterID string, contactServerList []string) (string, error) {
	// Extract the cluster index from the cluster ID (e.g., "C1" -> 1)
	clusterIndex, err := extractClusterIndex(clusterID)
	if err != nil {
		return "", err
	}

	// Ensure the index is within bounds of the ContactServerList
	if clusterIndex < 1 || clusterIndex > len(contactServerList) {
		return "", fmt.Errorf("invalid cluster index %d for ContactServerList", clusterIndex)
	}

	// Return the corresponding contact server
	return contactServerList[clusterIndex-1], nil
}

// extractClusterIndex extracts the numeric index from a cluster ID string
func extractClusterIndex(clusterID string) (int, error) {
	if !strings.HasPrefix(clusterID, "C") {
		return 0, fmt.Errorf("invalid cluster ID format: %s", clusterID)
	}
	return strconv.Atoi(clusterID[1:]) // Convert "C1" -> 1
}
