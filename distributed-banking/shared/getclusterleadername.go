package shared

import "fmt"

func GetClusterLeaderId(serverName string) string {
	// Extract the server number from the serverName
	var serverNumber int
	_, err := fmt.Sscanf(serverName, "S%d", &serverNumber)
	if err != nil {
		return "" // Return empty if the format is incorrect
	}

	// Determine the group based on the server number
	switch {
	case serverNumber >= 1 && serverNumber <= 4:
		return "S1"
	case serverNumber >= 5 && serverNumber <= 8:
		return "S5"
	case serverNumber >= 9 && serverNumber <= 12:
		return "S9"
	default:
		return "" // Return empty for numbers outside the specified ranges
	}
}
