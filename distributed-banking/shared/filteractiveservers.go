package shared

import "fmt"

// FilterActiveServers filters the active servers based on the provided cluster ID
func FilterActiveServers(activeServers []string, clusterID string) []string {
	var filteredServers []string
	for _, server := range activeServers {
		// Use the shared mapping to find the cluster of the server
		serverClusterID, err := GetClusterID(server)
		if err == nil && serverClusterID == clusterID {
			filteredServers = append(filteredServers, server)
		} else if err != nil {
			fmt.Printf("Error fetching cluster ID for server %s: %v\n", server, err)
		}
	}
	return filteredServers
}
