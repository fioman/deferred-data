package deferred

import (
	"errors"
	"sync"

	"github.com/reugn/async"
)

// 本地延迟数据
// 等待数据和设置数据的协程需要在同一个进程内
type LocalDeferred[T any] struct {
	promisesMap map[string]async.Promise[T]
	lock        sync.Mutex
}

func NewLocalDeferred[T any]() Deferred[T] {
	return &LocalDeferred[T]{
		promisesMap: make(map[string]async.Promise[T]),
	}
}

func (d *LocalDeferred[T]) Resolve(ticket string, value T) error {
	d.lock.Lock()
	defer d.lock.Unlock()
	if promise, ok := d.promisesMap[ticket]; ok {
		promise.Success(value)
		delete(d.promisesMap, ticket)
		return nil
	}
	return errors.New("ticket's promise not found")
}

func (d *LocalDeferred[T]) Reject(ticket string, err error) error {
	d.lock.Lock()
	defer d.lock.Unlock()
	if promise, ok := d.promisesMap[ticket]; ok {
		promise.Failure(err)
		delete(d.promisesMap, ticket)
		return nil
	}
	return errors.New("ticket's promise not found")
}

func (d *LocalDeferred[T]) Await(ticket string) (value T, err error) {
	promise := async.NewPromise[T]()
	d.lock.Lock()
	d.promisesMap[ticket] = promise
	d.lock.Unlock()
	return promise.Future().Get()
}
