package measurement

import "fmt"

var (
	ErrDBNotAvailable = fmt.Errorf("SQLite DB is not available")
)
