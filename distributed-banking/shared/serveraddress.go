package shared

import (
	"fmt"
	"strconv"
)

// ServerAddresses dynamically returns the localhost address for a given server ID
func ServerAddresses(serverID string) (string, error) {
	// Validate the server ID format (e.g., S<number>)
	if len(serverID) > 1 && serverID[0] == 'S' {
		// Parse the server number from the server ID
		serverNum, err := strconv.Atoi(serverID[1:])
		if err != nil {
			return "", fmt.Errorf("invalid server ID format: %s", serverID)
		}

		// Dynamically generate the address
		port := 5000 + serverNum
		address := fmt.Sprintf("localhost:%d", port)

		return address, nil
	}

	return "", fmt.Errorf("server ID not recognized: %s", serverID)
}
