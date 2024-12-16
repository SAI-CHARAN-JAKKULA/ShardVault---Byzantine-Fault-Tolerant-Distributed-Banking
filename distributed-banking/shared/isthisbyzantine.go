package shared

func IsByzantine(byzantineServerList []string, serverName string) bool {
	// Iterate through the list of Byzantine servers
	for _, byzantineServer := range byzantineServerList {
		// If we find a match, this server is Byzantine
		if byzantineServer == serverName {
			return true
		}
	}
	// Server was not found in Byzantine list
	return false
}
