package shared

var serversPerCluster int

func SetServersPerCluster(count int) {
	serversPerCluster = count
}

func GetQuorumSize() int {
	return (serversPerCluster + 1) / 2
}
