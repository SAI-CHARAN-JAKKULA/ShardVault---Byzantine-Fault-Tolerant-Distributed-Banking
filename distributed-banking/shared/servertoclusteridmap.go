package shared

import "fmt"

// ServerToClusterMapping maps server IDs to their corresponding cluster IDs
var ServerToClusterMapping = make(map[string]string)

// InitializeServerToClusterMapping initializes the server-to-cluster mapping
func InitializeServerToClusterMapping(clusterServers map[string][]string) {
	for clusterID, serverList := range clusterServers {
		for _, serverID := range serverList {
			ServerToClusterMapping[serverID] = clusterID
		}
	}
}

// GetClusterID retrieves the cluster ID for a given server ID
func GetClusterID(serverID string) (string, error) {
	clusterID, exists := ServerToClusterMapping[serverID]
	if !exists {
		return "", fmt.Errorf("server ID %s not found in mapping", serverID)
	}
	return clusterID, nil
}
