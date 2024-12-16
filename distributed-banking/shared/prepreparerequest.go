package shared

import "crypto/rsa"

type PrePrepareRequest struct {
	Transaction         Transaction
	ByzantineServerList []string
	PublicKey           *rsa.PublicKey
}
