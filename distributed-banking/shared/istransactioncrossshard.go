package shared

func IsTransactionCrossShard(source, destination int) bool {
	if (source >= 1 && source <= 1000 && destination >= 1 && destination <= 1000) ||
		(source >= 1001 && source <= 2000 && destination >= 1001 && destination <= 2000) ||
		(source >= 2001 && source <= 3000 && destination >= 2001 && destination <= 3000) {
		return false
	}
	return true
}
