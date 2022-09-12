package deferred

// 延迟数据管理
type Deferred[T any] interface {

	// 设置延迟得到的数据
	// ticket 是用户自定义的数据凭证
	// 设置后，Await 此 ticket 的协程将会返回数据
	Resolve(ticket string, value T) error

	// 设置延迟得到的错误
	// ticket 是用户自定义的数据凭证
	// 设置后，Await 此 ticket 的协程将会返回错误
	Reject(ticket string, err error) error

	// 等待数据
	// ticket 是用户自定义的数据凭证
	Await(ticket string) (value T, err error)
}
