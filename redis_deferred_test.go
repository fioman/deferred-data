package deferred

import (
	"fmt"
	"math/rand"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRedisDeferred(t *testing.T) {
	deferred, err := NewRedisDeferred(
		"test-redis-deferred",
		WithMarshal(func(i int) ([]byte, error) {
			return []byte(fmt.Sprintf("%d", i)), nil
		}),
		WithUnmarshal(func(b []byte, t *int) error {
			var err error
			*t, err = strconv.Atoi(string(b))
			return err
		}),
	)
	assert.Nil(t, err)

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
			// 至少要 1 秒后，确保先 await 再 resolve
			time.Sleep(time.Duration(1+rand.Intn(3)) * time.Second)
			deferred.Resolve(ticket, idx)
		}()
	}

	wg.Wait()
}
