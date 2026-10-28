package repository

import (
	"fmt"
	"sync/atomic"
)

var idCounter atomic.Uint64

func newID(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, idCounter.Add(1))
}
