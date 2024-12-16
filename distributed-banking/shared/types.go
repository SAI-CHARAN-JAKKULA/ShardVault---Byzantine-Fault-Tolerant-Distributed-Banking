package shared

type TwoPCArgs struct {
	Transaction    Transaction
	Role           string
	CrossShardRole string
}

type TwoPCReply struct {
	Success bool
}
