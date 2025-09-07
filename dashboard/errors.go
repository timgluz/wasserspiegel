package dashboard

import "fmt"

var (
	ErrKVStoreNotAvailable = fmt.Errorf("Spin KV store is not available")
)
