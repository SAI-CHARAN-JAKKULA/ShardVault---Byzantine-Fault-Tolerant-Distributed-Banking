package shared

// ServerStatus holds the status information for a server
type ServerStatus struct {
	BallotNumber int
	IsLeader     bool
}
