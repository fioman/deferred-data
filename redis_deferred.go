package deferred

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/avast/retry-go/v4"
	"github.com/gomodule/redigo/redis"
)

type RedisDeferred[T any] struct {
	name      string
	options   *RedisDeferredOptions[T]
	redisPool *redis.Pool
	channels  map[string]chan any
	lock      sync.RWMutex
}

type MarshalFunc[T any] func(T) ([]byte, error)
type UnmarshalFunc[T any] func([]byte, *T) error

type RedisDeferredOptions[T any] struct {
	marshal     MarshalFunc[T]   // 将 struct 序列化为字节数组
	unmarshal   UnmarshalFunc[T] // 将字节数组反序列化为 struct
	host        string           // redis连接
	password    string           // redis 认证密码
	maxIdle     int              // redis 连接池最大空闲连接
	maxActive   int              // redis 连接池最大连接数
	idleTimeout time.Duration    // redis 连接池空闲超时时间，超时后连接被回收
	db          int              // redis 选择的 db
	redisPool   *redis.Pool      // redis 连接池实例
}

type RedisDeferredOption[T any] func(*RedisDeferredOptions[T])

// 设置序列化函数
func WithMarshal[T any](marshal MarshalFunc[T]) RedisDeferredOption[T] {
	return func(rdo *RedisDeferredOptions[T]) {
		rdo.marshal = marshal
	}
}

// 设置反序列化函数
func WithUnmarshal[T any](unmarshal UnmarshalFunc[T]) RedisDeferredOption[T] {
	return func(rdo *RedisDeferredOptions[T]) {
		rdo.unmarshal = unmarshal
	}
}

// 设置 redis 连接地址
func WithHost[T any](host string) RedisDeferredOption[T] {
	return func(rdo *RedisDeferredOptions[T]) {
		rdo.host = host
	}
}

// 设置 redis 认证密码
func WithPassword[T any](password string) RedisDeferredOption[T] {
	return func(rdo *RedisDeferredOptions[T]) {
		rdo.password = password
	}
}

// 设置 redis 选择的 db
func WithDB[T any](db int) RedisDeferredOption[T] {
	return func(rdo *RedisDeferredOptions[T]) {
		rdo.db = db
	}
}

// 设置连接池，将直接使用此连接池，而不是自己创建
func WithPool[T any](redisPool *redis.Pool) RedisDeferredOption[T] {
	return func(rdo *RedisDeferredOptions[T]) {
		rdo.redisPool = redisPool
	}
}

// 设置连接池配置
func WithPoolOptions[T any](maxIdle, maxActive int, idleTimeout time.Duration) RedisDeferredOption[T] {
	return func(rdo *RedisDeferredOptions[T]) {
		rdo.maxIdle = maxIdle
		rdo.maxActive = maxActive
		rdo.idleTimeout = idleTimeout
	}
}

func NewRedisDeferred[T any](name string, options ...RedisDeferredOption[T]) (Deferred[T], error) {
	d := &RedisDeferred[T]{
		name: name,
		options: &RedisDeferredOptions[T]{
			marshal: func(t T) ([]byte, error) {
				return json.Marshal(t)
			},
			unmarshal: func(b []byte, t *T) error {
				return json.Unmarshal(b, t)
			},
			host:        "localhost:6379",
			maxIdle:     5,
			maxActive:   20,
			idleTimeout: 10 * time.Minute,
		},
		channels: make(map[string]chan any),
	}
	for _, opt := range options {
		opt(d.options)
	}
	if d.options.redisPool == nil {
		d.newPool()
	} else {
		d.redisPool = d.options.redisPool
	}
	d.poll()
	return d, nil
}

func (d *RedisDeferred[T]) newPool() {
	d.redisPool = &redis.Pool{
		MaxIdle:     d.options.maxIdle,
		MaxActive:   d.options.maxActive,
		IdleTimeout: d.options.idleTimeout,
		Wait:        true,
		Dial: func() (redis.Conn, error) {
			conn, err := redis.Dial("tcp", d.options.host)
			if err != nil {
				return nil, err
			}
			if d.options.db > 0 {
				if _, err = conn.Do("SELECT", d.options.db); err != nil {
					conn.Close()
					return nil, err
				}
			}
			if d.options.password != "" {
				if _, err := conn.Do("AUTH", d.options.password); err != nil {
					conn.Close()
					return nil, err
				}
			}
			return conn, nil
		},
		TestOnBorrow: func(conn redis.Conn, t time.Time) error {
			if _, err := conn.Do("PING"); err != nil {
				return err
			}
			return nil
		}}
}

func (d *RedisDeferred[T]) poll() {
	go func() {
		retry.Do(
			func() error {
				conn := d.redisPool.Get()
				defer conn.Close()

				psc := redis.PubSubConn{Conn: conn}
				if err := psc.Subscribe(redis.Args{}.Add(d.name)...); err != nil {
					return err
				}

				for {
					switch n := psc.Receive().(type) {
					case error:
						// 连接错误了，重新订阅
						return n
					case redis.Subscription:
						if n.Count == 0 {
							// 取消了订阅，需要重新订阅
							return fmt.Errorf("unsubscribe %s", d.name)
						}
					case redis.Message:
						result := &RedisDeferredResult{}
						if err := json.Unmarshal(n.Data, result); err != nil {
							continue
						}

						d.lock.RLock()
						channel, ok := d.channels[result.Ticket]
						d.lock.RUnlock()

						// 是其他进程在等待结果
						if !ok {
							continue
						}

						// 得到错误结果
						if result.Error != "" {
							channel <- errors.New(result.Error)
							continue
						}

						// 得到数据
						var value T
						if reflect.TypeOf(value) == nil {
							// 无法确定类型，只能将底层数据返回，交给业务自己处理
							channel <- result.Value
							continue
						}

						// 类型确定，可以转型
						if err := d.options.unmarshal(result.Value, &value); err != nil {
							channel <- err
						} else {
							channel <- value
						}
					}
				}
			},
			retry.Attempts(0),
			retry.Delay(1*time.Second),
			retry.DelayType(retry.FixedDelay),
		)
	}()
}

// 得到延迟数据结果
type RedisDeferredResult struct {
	Ticket string `json:"ticket,omitempty"` // 结果凭证
	Error  string `json:"error,omitempty"`  // 延迟错误
	Value  []byte `json:"value,omitempty"`  // 延迟数据
}

// 得到数据，发送给等待的进程
func (d *RedisDeferred[T]) Resolve(ticket string, value T) error {
	conn := d.redisPool.Get()
	defer conn.Close()

	valueBytes, err := d.options.marshal(value)
	if err != nil {
		return err
	}

	result, _ := json.Marshal(&RedisDeferredResult{
		Ticket: ticket,
		Value:  valueBytes,
	})
	_, err = conn.Do("PUBLISH", d.name, result)

	return err
}

// 得到错误，发送给等待的进程
func (d *RedisDeferred[T]) Reject(ticket string, err error) error {
	conn := d.redisPool.Get()
	defer conn.Close()

	result, _ := json.Marshal(&RedisDeferredResult{
		Ticket: ticket,
		Error:  err.Error(),
	})
	_, err = conn.Do("PUBLISH", d.name, result)

	return err
}

func (d *RedisDeferred[T]) Await(ticket string) (value T, err error) {
	channel := make(chan any)
	defer close(channel)

	d.lock.Lock()
	d.channels[ticket] = channel
	d.lock.Unlock()

	v := <-channel

	d.lock.Lock()
	delete(d.channels, ticket)
	d.lock.Unlock()

	switch result := v.(type) {
	case error:
		var t T
		return t, result
	default:
		return result.(T), nil
	}
}
