package deferred

import (
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestLocalDeferred(t *testing.T) {
	deferred := NewLocalDeferred[int]()
	var wg sync.WaitGroup

	for i := 0; i < 10000; i++ {
		wg.Add(1)
		idx := i
		ticket := fmt.Sprintf("%d", idx)

		go func() {
			val, err := deferred.Await(ticket)
			assert.Nil(t, err)
			assert.Equal(t, idx, val)
			wg.Done()
		}()

		go func() {
			time.Sleep(time.Duration(1+rand.Intn(3)) * time.Second)
			deferred.Resolve(ticket, idx)
		}()
	}

	wg.Wait()
}
